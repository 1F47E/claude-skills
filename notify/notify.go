package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout   = 120
	pollChunkSec     = 30
	replyCallback    = "__reply__"
	maxButtonsPerRow = 3
)

var (
	shortClient = &http.Client{Timeout: 15 * time.Second}
	longClient  = &http.Client{Timeout: time.Duration(pollChunkSec+15) * time.Second}
)

type tgResponse[T any] struct {
	Ok          bool   `json:"ok"`
	Description string `json:"description"`
	Result      T      `json:"result"`
}

type tgMessage struct {
	MessageID int `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Text string `json:"text"`
}

type tgCallbackQuery struct {
	ID      string    `json:"id"`
	Data    string    `json:"data"`
	Message tgMessage `json:"message"`
}

type tgUpdate struct {
	UpdateID      int              `json:"update_id"`
	Message       *tgMessage       `json:"message"`
	CallbackQuery *tgCallbackQuery `json:"callback_query"`
}

type tgButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

func main() {
	buttons := flag.String("buttons", "", "comma-separated Label:data pairs")
	timeout := flag.Int("timeout", defaultTimeout, "poll timeout in seconds")
	chatFlag := flag.String("chat", "", "Telegram chat ID (default: $CLAUDE_CODE_NOTIFY_CHAT_ID)")
	flag.Parse()

	token := os.Getenv("CLAUDE_CODE_NOTIFY_BOT_TOKEN")
	if token == "" {
		fatal("CLAUDE_CODE_NOTIFY_BOT_TOKEN not set")
	}

	chatID := *chatFlag
	if chatID == "" {
		chatID = os.Getenv("CLAUDE_CODE_NOTIFY_CHAT_ID")
	}
	if chatID == "" {
		fatal("CLAUDE_CODE_NOTIFY_CHAT_ID not set and --chat not provided")
	}
	if _, err := strconv.ParseInt(chatID, 10, 64); err != nil {
		fatal("invalid chat ID: " + chatID)
	}

	msg := strings.Join(flag.Args(), " ")
	if msg == "" {
		fatal("message required")
	}

	api := "https://api.telegram.org/bot" + token
	keyboard := parseButtons(*buttons)

	if err := deleteWebhook(api); err != nil {
		fmt.Fprintf(os.Stderr, "warn: deleteWebhook: %v\n", err)
	}

	if err := sendMessage(api, chatID, msg, keyboard); err != nil {
		fatal("send failed: " + err.Error())
	}
	fmt.Println("SENT: ok")

	offset, err := flushUpdates(api)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: flushUpdates: %v\n", err)
	}

	reply, err := pollForReply(api, chatID, offset, *timeout)
	if err != nil {
		fatal(err.Error())
	}
	if reply == "" {
		fmt.Printf("TIMEOUT: no response within %ds\n", *timeout)
	} else {
		fmt.Printf("REPLY: %s\n", reply)
	}
}

func parseButtons(spec string) [][]tgButton {
	var rows [][]tgButton
	if spec != "" {
		var row []tgButton
		for _, part := range strings.Split(spec, ",") {
			part = strings.TrimSpace(part)
			idx := strings.Index(part, ":")
			if idx < 0 {
				fmt.Fprintf(os.Stderr, "warn: skipping malformed button: %q\n", part)
				continue
			}
			label, data := part[:idx], part[idx+1:]
			if label == "" || data == "" {
				fmt.Fprintf(os.Stderr, "warn: skipping empty button label/data: %q\n", part)
				continue
			}
			row = append(row, tgButton{Text: label, CallbackData: data})
			if len(row) == maxButtonsPerRow {
				rows = append(rows, row)
				row = nil
			}
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	rows = append(rows, []tgButton{{Text: "✍️ Reply", CallbackData: replyCallback}})
	return rows
}

func deleteWebhook(api string) error {
	resp, err := shortClient.Get(api + "/deleteWebhook")
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

func sendMessage(api, chatID, text string, keyboard [][]tgButton) error {
	kb, _ := json.Marshal(map[string]any{"inline_keyboard": keyboard})
	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)
	params.Set("parse_mode", "HTML")
	params.Set("reply_markup", string(kb))

	var result tgResponse[tgMessage]
	return postForm(shortClient, api+"/sendMessage", params, &result)
}

func sendText(api, chatID, text string) {
	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)
	var result tgResponse[tgMessage]
	postForm(shortClient, api+"/sendMessage", params, &result)
}

func answerCallback(api, cbID string) {
	params := url.Values{}
	params.Set("callback_query_id", cbID)
	var result tgResponse[bool]
	postForm(shortClient, api+"/answerCallbackQuery", params, &result)
}

func flushUpdates(api string) (int, error) {
	params := url.Values{}
	params.Set("offset", "-1")
	params.Set("limit", "1")
	params.Set("allowed_updates", `["message","callback_query"]`)

	var result tgResponse[[]tgUpdate]
	if err := postForm(shortClient, api+"/getUpdates", params, &result); err != nil {
		return 0, err
	}
	if len(result.Result) > 0 {
		return result.Result[len(result.Result)-1].UpdateID + 1, nil
	}
	return 0, nil
}

func pollForReply(api, chatID string, offset, timeoutSec int) (string, error) {
	iterations := int(math.Ceil(float64(timeoutSec) / float64(pollChunkSec)))
	replyMode := false
	cid, _ := strconv.ParseInt(chatID, 10, 64)

	for i := 0; i < iterations; i++ {
		params := url.Values{}
		params.Set("offset", strconv.Itoa(offset))
		params.Set("timeout", strconv.Itoa(pollChunkSec))
		params.Set("allowed_updates", `["message","callback_query"]`)

		var result tgResponse[[]tgUpdate]
		if err := postForm(longClient, api+"/getUpdates", params, &result); err != nil {
			fmt.Fprintf(os.Stderr, "warn: poll: %v\n", err)
			continue
		}

		for _, upd := range result.Result {
			offset = upd.UpdateID + 1

			if cb := upd.CallbackQuery; cb != nil && cb.Message.Chat.ID == cid {
				answerCallback(api, cb.ID)

				if cb.Data == replyCallback {
					if !replyMode {
						sendText(api, chatID, "💬 Type your reply:")
						replyMode = true
					}
					continue
				}
				sendText(api, chatID, "👌 Got it: "+cb.Data)
				return cb.Data, nil
			}

			if msg := upd.Message; msg != nil && msg.Chat.ID == cid && replyMode {
				sendText(api, chatID, "👌 Got it: "+msg.Text)
				return msg.Text, nil
			}
		}
	}
	return "", nil
}

func postForm(client *http.Client, endpoint string, params url.Values, dest any) error {
	resp, err := client.PostForm(endpoint, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	type okChecker struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}
	var check okChecker
	if err := json.Unmarshal(body, &check); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	if !check.Ok {
		return fmt.Errorf("telegram: %s", check.Description)
	}
	return json.Unmarshal(body, dest)
}

func fatal(msg string) {
	fmt.Printf("ERROR: %s\n", msg)
	os.Exit(1)
}
