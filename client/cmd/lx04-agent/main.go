// Modified or added for XiaoSeek LX04, 2026-10-24. See NOTICE.
// SPDX-License-Identifier: Apache-2.0
// Standalone LX04 agent: native ASR, automatic routing, DeepSeek, native TTS.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"xiaobot"
	"xiaobot/deepseekweb"
	"xiaobot/devicevoice"
	"xiaobot/jarvis/openai"
	"xiaobot/runtimeenv"
	"xiaobot/voiceanswer"
)

type agent struct {
	mu         sync.Mutex
	gate       *devicevoice.AudioGate
	box        *xiaobot.XiaoMi
	client     *openai.Client
	answerer   *voiceanswer.Engine
	config     *xiaobot.Config
	doorbell   *doorbellGuard
	generation int
	cancel     context.CancelFunc
	pending    string
	lastQuery  string
	lastAt     time.Time
}

func (a *agent) restore() {
	if err := a.gate.Restore(); err != nil {
		log.Print("audio recovery failed; recovery file retained")
	}
}
func (a *agent) onSpeak(text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending != "" && strings.TrimSpace(text) == a.pending {
		a.pending = ""
		a.restore()
		log.Print("DeepSeek speech received locally; audio restored")
	}
}
func (a *agent) preemptForDoorbell() {
 a.mu.Lock()
 defer a.mu.Unlock()
 a.generation++
 if a.cancel!=nil {a.cancel()}
 a.pending=""
 a.restore()
 log.Print("AI cancelled for doorbell or unavailable receiver")
}
func (a *agent) onQuery(query string) {
 if a.doorbell!=nil && a.doorbell.check(context.Background()){a.preemptForDoorbell();return}
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if q == a.lastQuery && time.Since(a.lastAt) < 2*time.Second {
		return
	}
	a.lastQuery = q
	a.lastAt = time.Now()
	a.generation++
	generation := a.generation
	if a.cancel != nil {
		a.cancel()
	}
	a.pending = ""
	// "退出" is a control command, not a question for DeepSeek or a TTS answer.
	if q == "退出" || q == "退出。" || q == "退出！" || q == "退出!" || q == "退出播放" || q == "退出播放。" || q == "退出播放！" || q == "退出播放!" {
		a.restore()
		log.Print("route=exit; DeepSeek cancelled without speech or answer screen")
		return
	}
	if devicevoice.NativeRequest(q) {
		a.restore()
		log.Print("route=native")
		return
	}
	if err := a.gate.Mute(); err != nil {
		a.restore()
		log.Print("audio gate unavailable; native answer retained")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	a.cancel = cancel
	log.Print("route=deepseek")
	go a.answer(ctx, generation, q)
	// Bounded watchdog restores audio even if the network or native TTS stalls.
	go func() {
		<-ctx.Done()
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.generation == generation {
			a.pending = ""
			a.restore()
		}
	}()
}
func (a *agent) answer(ctx context.Context, generation int, query string) {
	started := time.Now()
	response, err := a.answerer.Answer(ctx, query)
	text := devicevoice.SpeechText(response.Text)
	if len(response.Searches) > 0 {
		data, _ := json.MarshalIndent(struct {
			Question string                     `json:"question"`
			At       time.Time                  `json:"at"`
			Searches []voiceanswer.SearchRecord `json:"searches"`
		}{query, time.Now(), response.Searches}, "", "  ")
		os.WriteFile(filepath.Join(filepath.Dir(a.gate.Path), "last-search.json"), data, 0600)
		log.Printf("web search completed: %d search calls", len(response.Searches))
	}
	if err != nil {
		text = "网络请求暂时没有成功，请稍后再试。"
		if errors.Is(err, deepseekweb.ErrSearch) {
			text = "这次联网搜索没有成功，暂时无法核实最新信息，请稍后再试。"
		}
		log.Print("answer request failed; preparing bounded spoken feedback")
	}
	if text == "" {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.generation == generation {
			a.restore()
		}
		return
	}
	if a.doorbell != nil && a.doorbell.check(ctx) {a.preemptForDoorbell();return}
	a.mu.Lock()
	if a.generation != generation || ctx.Err() != nil {
		a.mu.Unlock()
		return
	}
	a.pending = text
	a.mu.Unlock()
	log.Printf("DeepSeek answer ready in %s", time.Since(started).Round(time.Millisecond))
	showNativeAnswer(ctx, text)
	err = a.box.MiTTSContext(ctx, text)
	a.mu.Lock()
	if a.generation != generation {
		a.mu.Unlock()
		return
	}
	if err != nil {
		a.pending = ""
		a.restore()
		log.Print("native TTS request failed; audio restored")
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	// Fallback for a native build that does not log its Speak instruction.
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.generation == generation && a.pending != "" {
		a.pending = ""
		a.restore()
		log.Print("TTS accepted; audio restored by fallback")
	}
}

func main() {
	configPath := flag.String("config", "/data/adb/lx04-deepseek/config.json", "private config")
	restoreOnly := flag.Bool("restore-audio", false, "restore saved playback state and exit")
	flag.Parse()
	xiaobot.FixedZone()
	gate := &devicevoice.AudioGate{Path: filepath.Join(filepath.Dir(*configPath), ".audio-restore.json")}
	if err := gate.Restore(); err != nil {
		log.Fatal("could not restore saved audio state")
	}
	if *restoreOnly {
		return
	}
	if !devicevoice.SupportedAudioService() {
		log.Fatal("unsupported audio service; no audio changes made")
	}
	if runtimeenv.ConfigureDNSFromEnv() != nil {
		log.Fatal("invalid DNS configuration")
	}
	cfg, err := xiaobot.NewConfigFromFile(*configPath)
	if err != nil || cfg == nil || cfg.NeedSetup() || cfg.Hardware != "LX04" {
		log.Fatal("speaker configuration incomplete")
	}
	cfg.TokenPath = filepath.Join(filepath.Dir(*configPath), ".mi.token")
	box := xiaobot.NewXiaoMi(cfg)
	if box.InitMiBox() != nil {
		log.Fatal("Xiaomi session initialization failed")
	}
	client := openai.NewClient(cfg.OpenAIKey, cfg.OpenAIBackend, "")
	client.GPTOptions = cfg.GPTOptions
	answerer := &voiceanswer.Engine{Chat: client, Search: deepseekweb.NewClient(cfg.OpenAIKey), Model: cfg.Bot, Prompt: cfg.Prompt}
	a := &agent{gate: gate, box: box, client: client, config: cfg, answerer: answerer, doorbell: newDoorbellGuard("http://127.0.0.1:7979/state")}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	go a.doorbell.watch(ctx, a.preemptForDoorbell)
	go func() {
		<-ctx.Done()
		a.mu.Lock()
		if a.cancel != nil {
			a.cancel()
		}
		a.restore()
		a.mu.Unlock()
	}()
	defer a.restore()
	retrySpeech(ctx, a.listenSpeech, func() {
  a.mu.Lock()
  a.generation++
  if a.cancel!=nil {a.cancel()}
  a.pending=""
  a.restore()
  a.mu.Unlock()
 },3*time.Second)
}
