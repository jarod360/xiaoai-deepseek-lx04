// Standalone, one-shot LX04 MIoT TTS diagnostic. No agent or config changes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"xiaobot/miservice"
	"xiaobot/runtimeenv"
)

// The diagnostic must never rewrite the speaker's saved Xiaomi credentials.
type readOnlyTokens struct{ tokens *miservice.Tokens }

func (s readOnlyTokens) LoadToken() (*miservice.Tokens, error) { return s.tokens, nil }
func (s readOnlyTokens) SaveToken(*miservice.Tokens) error    { return nil }

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	configPath := flag.String("config", "/data/adb/lx04-deepseek/config.json", "speaker config path")
	speak := flag.Bool("speak", false, "send one short MIoT TTS test to the LX04")
	flag.Parse()
	if !*speak {
		fmt.Println("No request sent; pass -speak to send one short diagnostic announcement.")
		return
	}
	if err := runtimeenv.ConfigureDNSFromEnv(); err != nil {
		fail(err)
	}
	data, err := os.ReadFile(*configPath)
	if err != nil {
		fail(fmt.Errorf("could not read speaker config: %w", err))
	}
	var cfg struct {
		Hardware   string `json:"hardware"`
		Account    string `json:"account"`
		Password   string `json:"password"`
		DID        string `json:"mi_did"`
		UseCommand bool   `json:"use_command"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		fail(fmt.Errorf("invalid speaker config: %w", err))
	}
	if cfg.Hardware != "LX04" || !cfg.UseCommand || cfg.Account == "" || cfg.Password == "" || cfg.DID == "" {
		fail(fmt.Errorf("expected a configured LX04 with use_command=true; refusing request"))
	}
	tokenPath := filepath.Join(filepath.Dir(*configPath), ".mi.token")
	tokens, err := miservice.NewTokenStore(tokenPath).LoadToken()
	if err != nil || tokens == nil || tokens.UserName != cfg.Account {
		fail(fmt.Errorf("saved Xiaomi token missing or does not match configured account"))
	}
	account := miservice.NewAccount(cfg.Account, cfg.Password, readOnlyTokens{tokens: tokens})
	service := miservice.NewIOService(account, nil)
	code, err := service.MiotAction(cfg.DID, []int{5, 1}, []interface{}{"DeepSeek 播报诊断测试。"})
	if err != nil {
		fail(fmt.Errorf("LX04 MIoT 5-1 TTS request failed: %w", err))
	}
	if code != 0 {
		fail(fmt.Errorf("LX04 MIoT 5-1 TTS rejected (code %.0f)", code))
	}
	fmt.Println("LX04 MIoT 5-1 accepted (code 0). Check whether the speaker actually spoke.")
}
