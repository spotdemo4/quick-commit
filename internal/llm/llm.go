package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/spotdemo4/quick-commit/internal/tui"
)

type Llm struct {
	ollama *api.Client

	keywords []string
	model    string
	options  map[string]any

	output chan tui.Msg
}

func New(ctx context.Context, keywords []string, url *url.URL, model string, headers map[string]string, options map[string]any, output chan tui.Msg) (*Llm, error) {
	// Create http client
	client := &http.Client{
		Timeout: 5 * time.Minute,
		Transport: AuthMiddleware{
			Headers: headers,
			Proxied: http.DefaultTransport,
		},
	}

	// Create ollama
	ollama := api.NewClient(url, client)
	err := ollama.Heartbeat(ctx)
	if err != nil {
		return nil, err
	}

	return &Llm{
		ollama: ollama,

		keywords: keywords,
		model:    model,
		options:  options,

		output: output,
	}, nil
}

func (l *Llm) GenerateCommit(ctx context.Context, diff string, keyword string) (string, error) {
	// Set prompts
	systemPrompt := fmt.Sprintf(`You are to act as an author of a commit message in git. 
				You will be given the output of the 'git diff --staged' command, and you are to convert it into a commit message. 
				Craft a concise, single sentence commit message that encapsulates all changes made, with an emphasis on the primary updates. 
				If the modifications share a common theme or scope, mention it succinctly; otherwise, leave the scope out to maintain focus. 
				The goal is to provide a clear and unified overview of the changes in one single message.
				This is a '%s' commit, preface the commit with the conventional commit type '%s:'.`, keyword, keyword)
	prompt := fmt.Sprintf("%s\nHere is the output of 'git diff --staged': ```\n%s\n```", systemPrompt, diff)

	// Start loading spinner
	l.output <- tui.Msg{
		Type: tui.MsgLoading,
	}

	thoughts := ""
	response := ""
	thinking := false
	startTag := regexp.MustCompile("<(.*)>")
	var endTag *regexp.Regexp

	// Generate
	err := l.ollama.Generate(ctx, &api.GenerateRequest{
		Model:   l.model,
		Options: l.options,

		Prompt: prompt,
	},
		func(gr api.GenerateResponse) error {
			if thoughts == "" && strings.Contains(gr.Response, "<") {
				thinking = true
			}

			// This is a thought
			if thinking {
				thoughts += gr.Response

				// Start of thought
				if endTag == nil {
					tag := startTag.FindStringSubmatch(thoughts)
					if len(tag) >= 2 {
						endTag = regexp.MustCompile(fmt.Sprintf("</%s>", tag[1]))

						// Extract first thought
						split := strings.SplitN(thoughts, startTag.FindString(thoughts), 2)
						l.output <- tui.Msg{
							Text: split[1],
							Type: tui.MsgThought,
						}
					}

					return nil
				}

				// End of thought
				if endTag != nil && endTag.MatchString(thoughts) {
					thinking = false

					// Extract last thought & first response
					split := strings.SplitN(thoughts, endTag.FindString(thoughts), 2)
					response = split[1]
					l.output <- tui.Msg{
						Text: split[0],
						Type: tui.MsgThought,
					}
					l.output <- tui.Msg{
						Text: split[1],
						Type: tui.MsgResponse,
					}

					return nil
				}

				l.output <- tui.Msg{
					Text: gr.Response,
					Type: tui.MsgThought,
				}

				return nil
			}

			// This is a response
			response += gr.Response
			l.output <- tui.Msg{
				Text: gr.Response,
				Type: tui.MsgResponse,
			}

			return nil
		},
	)
	if err != nil {
		return "", err
	}

	// Check if context was cancelled
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Extract commit message from generated response
	commitMsg := ""
	lines := strings.Split(response, "\n")

	// Check if lines contains specified keyword
	for i := range lines {
		line := lines[len(lines)-i-1] // backwards -> forwards

		if strings.Contains(line, keyword) {
			_, after, _ := strings.Cut(line, keyword)
			commitMsg = keyword + after

			break
		}
	}

	// If not yet found, check all keywords
	if commitMsg == "" {
	keywords:
		for i := range lines {
			line := lines[len(lines)-i-1] // backwards -> forwards

			for _, keyword := range l.keywords {
				if strings.Contains(line, keyword) {
					_, after, _ := strings.Cut(line, keyword)
					commitMsg = keyword + after

					break keywords
				}
			}
		}
	}

	if commitMsg == "" {
		return "", errors.New("no commit message found")
	}

	// Remove special characters
	commitMsg = strings.ReplaceAll(commitMsg, "`", "")
	commitMsg = strings.ReplaceAll(commitMsg, "*", "")

	// Trim whitespace
	commitMsg = strings.TrimSpace(commitMsg)

	return commitMsg, nil
}

type AuthMiddleware struct {
	Headers map[string]string
	Proxied http.RoundTripper
}

func (am AuthMiddleware) RoundTrip(req *http.Request) (res *http.Response, e error) {
	for k, v := range am.Headers {
		req.Header.Add(k, v)
	}

	return am.Proxied.RoundTrip(req)
}
