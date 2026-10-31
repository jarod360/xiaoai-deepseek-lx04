package miservice

// Helpers for comparing MIoT RC4 payload formats offline.
// No network requests or credentials are handled here.
import (
    "crypto/rc4"
    "crypto/sha1"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "net/url"
    "strings"
)
func miGPTSHA1(parts ...string) string {
    sum := sha1.Sum([]byte(strings.Join(parts, "&")))
    return base64.StdEncoding.EncodeToString(sum[:])
}
func miGPTRC4(key []byte, pieces ...[]byte) ([][]byte, error) {
    c, err := rc4.NewCipher(key)
    if err != nil { return nil, err }
    discard := make([]byte, 1024)
    c.XORKeyStream(discard, discard)
    out := make([][]byte, 0, len(pieces))
    for _, part := range pieces {
        b := append([]byte(nil), part...)
        c.XORKeyStream(b, b)
        out = append(out, b)
    }
    return out, nil
}
func miGPTEncodeMIoT(path string, params any, ssecurity, nonce string) (url.Values, error) {
    payload, err := json.Marshal(map[string]any{"params": params, "datasource": 2})
    if err != nil { return nil, err }
    snonce, err := signNonce(ssecurity, nonce)
    if err != nil { return nil, err }
    key, err := base64.StdEncoding.DecodeString(snonce)
    if err != nil { return nil, err }
    innerHash := miGPTSHA1("POST", path, "data="+string(payload), snonce)
    chunks, err := miGPTRC4(key, payload, []byte(innerHash))
    if err != nil { return nil, err }
    encryptedData := base64.StdEncoding.EncodeToString(chunks[0])
    encryptedHash := base64.StdEncoding.EncodeToString(chunks[1])
    signature := miGPTSHA1("POST", path, fmt.Sprintf("data=%s", encryptedData), fmt.Sprintf("rc4_hash__=%s", encryptedHash), snonce)
    return url.Values{"data": {encryptedData}, "rc4_hash__": {encryptedHash}, "signature": {signature}, "_nonce": {nonce}, "ssecurity": {ssecurity}}, nil
}
