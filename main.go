package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	showcaseSlug = "war_robots"
	baseURL      = "https://market.my.games"
	authURL      = "https://auth-ac.my.games"
	userAgent    = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

type Config struct {
	Name     string
	ID       string
	Password string
}

// Load .env configuration
func loadEnv() Config {
	cfg := Config{}

	// Paths to check for .env
	home, _ := os.UserHomeDir()
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	paths := []string{
		".env",
		filepath.Join(exeDir, ".env"),
		filepath.Join(home, ".config/war-robots-autoclaim/.env"),
		filepath.Join(home, "Documents/ngatur laptop/.env"),
		"/home/joo/war-robots-autoclaim/.env",
	}

	var content []byte
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			content = data
			log.Printf("[+] Loaded .env from: %s", p)
			break
		}
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch strings.ToLower(key) {
		case "name":
			cfg.Name = val
		case "id":
			cfg.ID = val
		case "pass", "password":
			cfg.Password = val
		}
	}

	return cfg
}

func getSessionFile() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config/war-robots-autoclaim")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "session.json")
}

func loadSavedCookies() map[string]string {
	data, err := os.ReadFile(getSessionFile())
	if err != nil {
		return nil
	}
	var cookies map[string]string
	if err := json.Unmarshal(data, &cookies); err == nil {
		return cookies
	}
	return nil
}

func saveCookies(cookies map[string]string) {
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err == nil {
		_ = os.WriteFile(getSessionFile(), data, 0600)
	}
}

// Derive PBKDF2 HMAC-SHA1 key for Chromium Linux (1 iteration, 16 bytes)
func derivePBKDF2Key(password []byte) []byte {
	salt := []byte("saltysalt")
	data := append(salt, 0, 0, 0, 1)
	h := hmac.New(sha1.New, password)
	h.Write(data)
	return h.Sum(nil)[:16]
}

// Decrypt Chromium/Brave cookie
func decryptCookie(enc []byte, key []byte) (string, error) {
	if len(enc) < 3 {
		return "", fmt.Errorf("cookie data too short")
	}
	prefix := string(enc[:3])
	ciphertext := enc[3:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext length is not a multiple of block size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	iv := bytes.Repeat([]byte(" "), 16)
	mode := cipher.NewCBCDecrypter(block, iv)
	dec := make([]byte, len(ciphertext))
	mode.CryptBlocks(dec, ciphertext)

	if len(dec) == 0 {
		return "", fmt.Errorf("empty decrypted content")
	}
	pad := int(dec[len(dec)-1])
	if pad <= 0 || pad > len(dec) {
		return "", fmt.Errorf("invalid padding")
	}
	unpadded := dec[:len(dec)-pad]

	if prefix == "v11" && len(unpadded) > 32 {
		return string(unpadded[32:]), nil
	}
	return string(unpadded), nil
}

// Extract cookies from local Brave browser if available
func extractBraveCookies() (map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cookiePath := filepath.Join(home, ".config/BraveSoftware/Brave-Browser/Default/Cookies")
	if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Brave cookies database not found at %s", cookiePath)
	}

	var bravePass string
	cmd := exec.Command("kwallet-query", "-f", "Brave Keys", "-r", "Brave Safe Storage", "kdewallet")
	if out, err := cmd.Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		bravePass = strings.TrimSpace(string(out))
	} else {
		cmd2 := exec.Command("kwallet-query", "-r", "Brave Safe Storage", "kdewallet")
		if out2, err2 := cmd2.Output(); err2 == nil && len(strings.TrimSpace(string(out2))) > 0 {
			bravePass = strings.TrimSpace(string(out2))
		}
	}

	if bravePass == "" {
		return nil, fmt.Errorf("could not retrieve Brave Safe Storage key from KWallet")
	}

	key := derivePBKDF2Key([]byte(bravePass))

	sqlCmd := exec.Command("sqlite3", cookiePath,
		"SELECT name, hex(encrypted_value) FROM cookies WHERE host_key LIKE '%my.games%';")
	sqlOut, err := sqlCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sqlite3 query error: %w", err)
	}

	cookies := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(sqlOut)), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		rawHex := strings.TrimSpace(parts[1])
		enc, err := hex.DecodeString(rawHex)
		if err != nil {
			continue
		}
		val, err := decryptCookie(enc, key)
		if err == nil && len(val) > 0 {
			cookies[name] = val
		}
	}

	return cookies, nil
}

func buildClientWithCookies(cookies map[string]string) *http.Client {
	jar, _ := cookiejar.New(nil)
	uMarket, _ := url.Parse("https://market.my.games")
	uMyGames, _ := url.Parse("https://my.games")
	uAuth, _ := url.Parse("https://auth-ac.my.games")

	var cookiesMarket []*http.Cookie
	var cookiesMyGames []*http.Cookie

	for k, v := range cookies {
		c := &http.Cookie{
			Name:   k,
			Value:  v,
			Domain: ".my.games",
			Path:   "/",
		}
		cookiesMyGames = append(cookiesMyGames, c)
		cookiesMarket = append(cookiesMarket, &http.Cookie{
			Name:   k,
			Value:  v,
			Domain: "market.my.games",
			Path:   "/",
		})
	}
	jar.SetCookies(uMarket, cookiesMarket)
	jar.SetCookies(uMyGames, cookiesMyGames)
	jar.SetCookies(uAuth, cookiesMyGames)

	return &http.Client{
		Jar:     jar,
		Timeout: 20 * time.Second,
	}
}

// Perform direct email+password login against MY.GAMES auth API
func loginMyGames(id, password string) (map[string]string, error) {
	log.Printf("[*] Initiating MY.GAMES login for ID: %s...", id)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 20 * time.Second,
	}

	// 1. Auth Init
	initPayload, _ := json.Marshal(map[string]any{
		"login":    id,
		"continue": "https://market.my.games/war_robots?content=items",
	})
	reqInit, _ := http.NewRequest("POST", authURL+"/api/v3/pub/auth/init", bytes.NewBuffer(initPayload))
	reqInit.Header.Set("Content-Type", "application/json")
	reqInit.Header.Set("User-Agent", userAgent)

	respInit, err := client.Do(reqInit)
	if err != nil {
		return nil, fmt.Errorf("auth_init request failed: %w", err)
	}
	defer respInit.Body.Close()

	if respInit.StatusCode != 200 {
		body, _ := io.ReadAll(respInit.Body)
		return nil, fmt.Errorf("auth_init HTTP %d: %s", respInit.StatusCode, string(body))
	}

	var initResp struct {
		Token        string `json:"token"`
		NeedPassword bool   `json:"need_password"`
	}
	json.NewDecoder(respInit.Body).Decode(&initResp)
	if initResp.Token == "" {
		return nil, fmt.Errorf("auth_init did not return token")
	}

	// 2. Auth Verify with Password
	verifyPayload, _ := json.Marshal(map[string]any{
		"login":    id,
		"token":    initResp.Token,
		"password": password,
		"continue": "https://market.my.games/war_robots?content=items",
	})
	reqVerify, _ := http.NewRequest("POST", authURL+"/api/v3/pub/auth/verify", bytes.NewBuffer(verifyPayload))
	reqVerify.Header.Set("Content-Type", "application/json")
	reqVerify.Header.Set("User-Agent", userAgent)

	respVerify, err := client.Do(reqVerify)
	if err != nil {
		return nil, fmt.Errorf("auth_verify request failed: %w", err)
	}
	defer respVerify.Body.Close()

	var verifyResp struct {
		AuthRedirect string `json:"auth_redirect"`
		Error        string `json:"error"`
	}
	json.NewDecoder(respVerify.Body).Decode(&verifyResp)

	if verifyResp.AuthRedirect == "" {
		return nil, fmt.Errorf("auth_verify failed (check credentials): %s", verifyResp.Error)
	}

	// 3. Follow auth_redirect to set session cookies
	reqRedir, _ := http.NewRequest("GET", verifyResp.AuthRedirect, nil)
	reqRedir.Header.Set("User-Agent", userAgent)
	respRedir, err := client.Do(reqRedir)
	if err != nil {
		return nil, fmt.Errorf("auth_redirect request failed: %w", err)
	}
	respRedir.Body.Close()

	// 4. Request market.my.games/api/initial/ to set csrftoken_v2 & market cookies
	reqMarketInit, _ := http.NewRequest("GET", baseURL+"/api/initial/", nil)
	reqMarketInit.Header.Set("User-Agent", userAgent)
	respMarketInit, err := client.Do(reqMarketInit)
	if err != nil {
		return nil, fmt.Errorf("market initial failed: %w", err)
	}
	respMarketInit.Body.Close()

	// 5. Gather all cookies from jar
	allCookies := make(map[string]string)
	for _, domainStr := range []string{"https://my.games", "https://market.my.games", "https://auth-ac.my.games"} {
		u, _ := url.Parse(domainStr)
		for _, c := range jar.Cookies(u) {
			allCookies[c.Name] = c.Value
		}
	}

	log.Printf("[+] Login successful! Captured %d session cookies.", len(allCookies))
	return allCookies, nil
}

// Verify if current session is authenticated as target player name
func checkSession(client *http.Client, targetName string, csrf string) (bool, *AccountBinding) {
	url := fmt.Sprintf("%s/api/showcases/163/account_bindings/", baseURL)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	if csrf != "" {
		req.Header.Set("X-Csrftoken", csrf)
	}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return false, nil
	}
	defer resp.Body.Close()

	var bindings AccountBindingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&bindings); err != nil {
		return false, nil
	}

	targetAcc := bindings.DefaultAccount
	if targetAcc.UserID == "" && len(bindings.PreSavedAccounts) > 0 {
		targetAcc = bindings.PreSavedAccounts[0]
	}

	if targetAcc.UserID != "" {
		if targetName == "" || strings.EqualFold(targetAcc.Nickname, targetName) {
			return true, &targetAcc
		}
	}

	return false, nil
}

func notify(title, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "notify-send", "-a", "War Robots Auto-Claimer", title, message).Run()
}

type ShopItem struct {
	ID     any    `json:"id"`
	Title  string `json:"title"`
	IsFree bool   `json:"isFree"`
}

type ShopItemsResponse struct {
	Results []ShopItem `json:"results"`
}

type AccountBinding struct {
	AppID    string `json:"appId"`
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
}

type AccountBindingsResponse struct {
	DefaultAccount   AccountBinding   `json:"defaultAccount"`
	PreSavedAccounts []AccountBinding `json:"preSavedAccounts"`
}

type BindingTokenResponse struct {
	Token    string `json:"token"`
	Nickname string `json:"nickname"`
}

func main() {
	log.Println("==================================================")
	log.Println("🤖 War Robots Auto-Claimer (Go Edition) Starting...")
	log.Println("==================================================")

	cfg := loadEnv()

	var cookies map[string]string
	var client *http.Client
	var targetAcc *AccountBinding

	// 1. Try saved session
	cookies = loadSavedCookies()
	if len(cookies) > 0 {
		client = buildClientWithCookies(cookies)
		if ok, acc := checkSession(client, cfg.Name, cookies["csrftoken_v2"]); ok {
			log.Printf("[+] Verified existing session for player '%s'.", acc.Nickname)
			targetAcc = acc
		} else {
			log.Println("[-] Saved session expired or not matching target user.")
			cookies = nil
		}
	}

	// 2. If not logged in, try logging in using .env credentials
	if cookies == nil && cfg.ID != "" && cfg.Password != "" {
		var loginErr error
		for attempt := 1; attempt <= 5; attempt++ {
			cookies, loginErr = loginMyGames(cfg.ID, cfg.Password)
			if loginErr == nil && len(cookies) > 0 {
				saveCookies(cookies)
				client = buildClientWithCookies(cookies)
				if ok, acc := checkSession(client, cfg.Name, cookies["csrftoken_v2"]); ok {
					targetAcc = acc
					break
				}
			}
			log.Printf("[-] Login attempt %d failed: %v. Retrying in 4s...", attempt, loginErr)
			time.Sleep(4 * time.Second)
		}
	}

	// 3. Fallback: extract from local Brave browser if still not logged in
	if cookies == nil {
		log.Println("[*] Attempting fallback to local Brave browser cookies...")
		for attempt := 1; attempt <= 5; attempt++ {
			cookies, _ = extractBraveCookies()
			if len(cookies) > 0 {
				client = buildClientWithCookies(cookies)
				if ok, acc := checkSession(client, cfg.Name, cookies["csrftoken_v2"]); ok {
					targetAcc = acc
					saveCookies(cookies)
					break
				}
			}
			time.Sleep(2 * time.Second)
		}
	}

	if cookies == nil || targetAcc == nil {
		log.Fatalf("[-] Authentication failed. Please check your .env credentials (id=%s).", cfg.ID)
	}

	log.Printf("[+] Authenticated Player: %s (%s, AppID: %s)", targetAcc.Nickname, targetAcc.UserID, targetAcc.AppID)

	showcaseID := 163 // War Robots showcase
	csrf := cookies["csrftoken_v2"]

	// 4. Fetch Shop Items Catalog
	itemsURL := fmt.Sprintf("%s/api/showcases/%d/shop_items/", baseURL, showcaseID)
	reqItems, _ := http.NewRequest("GET", itemsURL, nil)
	reqItems.Header.Set("User-Agent", userAgent)
	respItems, err := client.Do(reqItems)
	if err != nil || respItems.StatusCode != 200 {
		log.Fatalf("[-] Failed to fetch shop catalog: %v", err)
	}
	defer respItems.Body.Close()

	var catalog ShopItemsResponse
	if err := json.NewDecoder(respItems.Body).Decode(&catalog); err != nil {
		log.Fatalf("[-] Failed to decode catalog: %v", err)
	}

	var freeItems []ShopItem
	for _, item := range catalog.Results {
		if item.IsFree {
			freeItems = append(freeItems, item)
		}
	}

	if len(freeItems) == 0 {
		log.Println("[-] No free items found currently in War Robots market.")
		return
	}

	log.Printf("[+] Found %d free item(s) in catalog:", len(freeItems))
	for _, fi := range freeItems {
		log.Printf("    * ID: %v | Name: %s", fi.ID, fi.Title)
	}

	// 5. Claim Each Free Item
	for _, targetItem := range freeItems {
		productIDStr := fmt.Sprintf("%v", targetItem.ID)
		log.Printf("[*] Processing claim for: %s (ID: %s)...", targetItem.Title, productIDStr)

		// 5a. Request binding token
		tokenURL := fmt.Sprintf("%s/api/account_bindings/%s/%s/", baseURL, targetAcc.AppID, targetAcc.UserID)
		tokenReqBody, _ := json.Marshal(map[string]any{
			"remember":   true,
			"productId":  productIDStr,
			"showcaseId": showcaseID,
		})
		reqToken, _ := http.NewRequest("POST", tokenURL, bytes.NewBuffer(tokenReqBody))
		reqToken.Header.Set("Content-Type", "application/json")
		reqToken.Header.Set("User-Agent", userAgent)
		reqToken.Header.Set("Origin", baseURL)
		reqToken.Header.Set("Referer", fmt.Sprintf("%s/%s?content=items", baseURL, showcaseSlug))
		reqToken.Header.Set("X-Csrftoken", csrf)

		respToken, err := client.Do(reqToken)
		if err != nil || respToken.StatusCode != 200 {
			log.Printf("[-] Failed to get account binding token: status %d", respToken.StatusCode)
			continue
		}

		var tokenData BindingTokenResponse
		json.NewDecoder(respToken.Body).Decode(&tokenData)
		respToken.Body.Close()

		if tokenData.Token == "" {
			log.Printf("[-] Empty token received from account bindings.")
			continue
		}

		// 5b. Submit order to claim free item
		orderURL := fmt.Sprintf("%s/api/orders/", baseURL)
		orderPayload, _ := json.Marshal(map[string]any{
			"showcaseId":    showcaseID,
			"productId":     productIDStr,
			"quantity":      1,
			"kind":          "showcase",
			"mrgsAccountId": tokenData.Token,
		})

		reqOrder, _ := http.NewRequest("POST", orderURL, bytes.NewBuffer(orderPayload))
		reqOrder.Header.Set("Content-Type", "application/json")
		reqOrder.Header.Set("User-Agent", userAgent)
		reqOrder.Header.Set("Origin", baseURL)
		reqOrder.Header.Set("Referer", fmt.Sprintf("%s/%s?content=items", baseURL, showcaseSlug))
		reqOrder.Header.Set("X-Csrftoken", csrf)

		respOrder, err := client.Do(reqOrder)
		if err != nil {
			log.Printf("[-] Order request failed: %v", err)
			continue
		}

		bodyBytes, _ := io.ReadAll(respOrder.Body)
		respOrder.Body.Close()

		if respOrder.StatusCode == 200 {
			log.Printf("[SUCCESS] Successfully claimed: %s! Response: %s", targetItem.Title, string(bodyBytes))
			notify("War Robots Auto-Claimer", fmt.Sprintf("🎁 Successfully claimed %s for %s!", targetItem.Title, targetAcc.Nickname))
		} else if respOrder.StatusCode == 422 && strings.Contains(string(bodyBytes), "product_limit_exceeded") {
			log.Printf("[INFO] %s has already been claimed today.", targetItem.Title)
			notify("War Robots Auto-Claimer", fmt.Sprintf("ℹ️ %s already claimed today.", targetItem.Title))
		} else {
			log.Printf("[-] Claim order failed (HTTP %d): %s", respOrder.StatusCode, string(bodyBytes))
		}
	}

	log.Println("[+] Finished War Robots auto-claim task.")
}
