package scraper

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	loginURL    = "https://account-api.bandainamcoid.com/v3/login/idpw"
	redirectURI = "https://www.bandainamcoid.com/v2/oauth2/auth?back=v3&client_id=gundamexvs&scope=JpGroupAll&redirect_uri=https%3A%2F%2Fweb.vsmobile.jp%2Fexvs2ib%2Fregist&text="
)

// Client はHTTPクライアント
type Client struct {
	Username   string
	Password   string
	HTTPClient *http.Client
}

type loginResponse struct {
	Status string `json:"result"`
	Cookie struct {
		RetentionTmp struct {
			Name    string `json:"name"`
			Value   string `json:"value"`
			Expires int    `json:"expires"`
		} `json:"retention_tmp"`
		DeleteLogin struct {
			Name string `json:"name"`
		} `json:"delete_login"`
		DeleteLoginCheck struct {
			Name string `json:"name"`
		} `json:"delete_login_check"`
		DeleteCommon struct {
			Name   string `json:"name"`
			Path   string `json:"path"`
			Domain string `json:"domain"`
		} `json:"delete_common"`
		Login struct {
			Name    string `json:"name"`
			Value   string `json:"value"`
			Expires int    `json:"expires"`
		} `json:"login"`
		LoginCheck struct {
			Name    string `json:"name"`
			Value   string `json:"value"`
			Expires int    `json:"expires"`
		} `json:"login_check"`
		Common struct {
			Name    string `json:"name"`
			Value   string `json:"value"`
			Expires int    `json:"expires"`
			Path    string `json:"path"`
			Domain  string `json:"domain"`
		} `json:"common"`
		Mnw struct {
			Name    string `json:"name"`
			Value   string `json:"value"`
			Expires int    `json:"expires"`
			Path    string `json:"path"`
			Domain  string `json:"domain"`
		} `json:"mnw"`
		Shortcut struct {
			Name string `json:"name"`
		} `json:"shortcut"`
		Retention struct {
			Name    string `json:"name"`
			Value   string `json:"value"`
			Expires int    `json:"expires"`
		} `json:"retention"`
	} `json:"cookie"`
	Data struct {
		View struct {
			PrivacyPolicy struct {
				URL string `json:"url"`
			} `json:"privacy_policy"`
			GlobalConcent struct {
				Text string `json:"text"`
				Flag string `json:"flag"`
			} `json:"global_concent"`
			Terms struct {
				Text string `json:"text"`
			} `json:"terms"`
		} `json:"view"`
	} `json:"data"`
	RedirectUrl string `json:"redirect"`
}

// NewClient は新しいクライアントを作成する
func NewClient(username, password string) *Client {
	cookieJar, _ := cookiejar.New(nil)

	c := &Client{
		Username: username,
		Password: password,
	}

	c.HTTPClient = &http.Client{
		Transport:     http.DefaultTransport,
		CheckRedirect: http.DefaultClient.CheckRedirect,
		Jar:           cookieJar,
		Timeout:       30 * time.Second,
	}

	return c
}

// Login はバンダイナムコIDでログインする
func (c *Client) Login() error {
	v := url.Values{}
	v.Set("client_id", "gundamexvs")
	v.Set("redirect_uri", redirectURI)
	v.Set("customize_id", "")
	v.Set("login_id", c.Username)
	v.Set("password", c.Password)
	v.Set("shortcut", "0")
	v.Set("retention", "0")
	v.Set("language", "ja")
	v.Set("cookie", `{"language":"ja"}`)
	v.Set("prompt", "")

	loginPage, err := c.HTTPClient.PostForm(loginURL, v)
	if err != nil {
		log.Fatal(err)
	}
	defer loginPage.Body.Close()

	var l loginResponse
	err = json.NewDecoder(loginPage.Body).Decode(&l)
	if err != nil {
		log.Fatal(err)
	}

	if strings.Contains(l.RedirectUrl, "passkey") {
		err = c.skipPasskey(l)
	} else {
		authPage, err := c.HTTPClient.Get(l.RedirectUrl)
		if err != nil {
			log.Fatal(err)
		}
		defer authPage.Body.Close()
	}

	return err
}

func (c *Client) skipPasskey(l loginResponse) error {
	parsedURL, err := url.Parse(l.RedirectUrl)
	if err != nil {
		return err
	}
	q := parsedURL.Query()

	cookieJSON := map[string]string{"language": "ja"}
	if l.Cookie.Login.Name != "" {
		cookieJSON[l.Cookie.Login.Name] = l.Cookie.Login.Value
	}
	if l.Cookie.LoginCheck.Name != "" {
		cookieJSON[l.Cookie.LoginCheck.Name] = l.Cookie.LoginCheck.Value
	}
	if l.Cookie.Common.Name != "" {
		cookieJSON[l.Cookie.Common.Name] = l.Cookie.Common.Value
	}
	if l.Cookie.Mnw.Name != "" {
		cookieJSON[l.Cookie.Mnw.Name] = l.Cookie.Mnw.Value
	}
	if l.Cookie.Retention.Name != "" {
		cookieJSON[l.Cookie.Retention.Name] = l.Cookie.Retention.Value
	}
	if l.Cookie.RetentionTmp.Name != "" {
		cookieJSON[l.Cookie.RetentionTmp.Name] = l.Cookie.RetentionTmp.Value
	}
	cookieBytes, _ := json.Marshal(cookieJSON)

	params := url.Values{}
	params.Set("client_id", q.Get("client_id"))
	params.Set("backto", q.Get("backto"))
	params.Set("redirect_uri", q.Get("redirect_uri"))
	params.Set("customize_id", q.Get("customize_id"))
	params.Set("code", q.Get("code"))
	params.Set("language", "ja")
	params.Set("cookie", string(cookieBytes))

	passkeyInfoURL := "https://account-api.bandainamcoid.com/v3/passkey/info?" + params.Encode()
	skipResp, err := c.HTTPClient.Get(passkeyInfoURL)
	if err != nil {
		return err
	}
	defer skipResp.Body.Close()

	var passkeyResp map[string]interface{}
	if err := json.NewDecoder(skipResp.Body).Decode(&passkeyResp); err != nil {
		return err
	}

	redirectURL := ""
	if data, ok := passkeyResp["data"].(map[string]interface{}); ok {
		if btn, ok := data["btn"].(map[string]interface{}); ok {
			if btnNext, ok := btn["btn-next"].(map[string]interface{}); ok {
				if u, ok := btnNext["url"].(string); ok {
					redirectURL = u
				}
			}
		}
	}

	if redirectURL == "" {
		log.Fatal("passkey/info APIからリダイレクトURLを取得できませんでした")
	}

	if cookie, ok := passkeyResp["cookie"].(map[string]interface{}); ok {
		if pi, ok := cookie["passkey_info"].(map[string]interface{}); ok {
			if name, ok := pi["name"].(string); ok {
				if value, ok := pi["value"].(string); ok {
					accountURL, _ := url.Parse("https://account.bandainamcoid.com/")
					c.HTTPClient.Jar.SetCookies(accountURL, []*http.Cookie{{Name: name, Value: value}})
				}
			}
		}
	}

	bnidURL, _ := url.Parse("https://www.bandainamcoid.com/")
	c.HTTPClient.Jar.SetCookies(bnidURL, []*http.Cookie{
		{Name: l.Cookie.Common.Name, Value: l.Cookie.Common.Value, Domain: ".bandainamcoid.com", Path: "/"},
		{Name: l.Cookie.Mnw.Name, Value: l.Cookie.Mnw.Value, Domain: ".bandainamcoid.com", Path: "/"},
	})

	authPage, err := c.HTTPClient.Get(redirectURL)
	if err != nil {
		return err
	}
	defer authPage.Body.Close()

	return nil
}

// NewCookieJar はログイン済みのCookieJarを返す
func NewCookieJar(username, password string) (http.CookieJar, error) {
	c := NewClient(username, password)
	err := c.Login()
	if err != nil {
		log.Fatal(err)
	}
	return c.HTTPClient.Jar, err
}
