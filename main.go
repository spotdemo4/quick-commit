package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/ollama/ollama/api"
)

func main() {
	// Get .env file
	configDir, err := os.UserConfigDir()
	if err != nil {
		printWarn("warning: could not get config dir: %v", err)
	} else {
		err := godotenv.Load(filepath.Join(configDir, "quick-commit.env"))
		if err != nil {
			printWarn("warning: could not load %s", filepath.Join(configDir, "quick-commit.env"))
		}
	}

	// Get env vars
	urlStr := os.Getenv("QC_URL")
	if urlStr == "" {
		urlStr := "http://localhost:11434"
		printWarn("warning: 'QC_URL' not set, defaulting to %s", urlStr)
	}
	url, err := url.Parse(urlStr)
	if err != nil {
		printErr("error: could not parse url: %v", err)
	}

	model := os.Getenv("QC_MODEL")
	if model == "" {
		printErr("error: 'QC_MODEL' not set")
	}

	// Get headers and options
	headers := map[string]string{}
	options := map[string]any{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "QC_HEADER_") {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) != 2 {
				continue
			}

			headers[strings.TrimPrefix(kv[0], "QC_HEADER_")] = kv[1]
			continue
		}

		if strings.HasPrefix(e, "QC_OPTION_") {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) != 2 {
				continue
			}

			opt := strings.TrimPrefix(kv[0], "QC_OPTION_")

			if opt == "temperature" {
				float, err := strconv.ParseFloat(kv[1], 32)
				if err != nil {
					printErr("error, invalid value for 'QC_OPTION_TEMPERATURE': %v", err)
				}

				options[opt] = float32(float)
				continue
			}

			options[strings.TrimPrefix(kv[0], "QC_OPTION_")] = kv[1]
			continue
		}
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	// Create channels
	msgChan := make(chan Msg, 10)
	retryChan := make(chan int, 1)

	// Create tea
	t := tea.NewProgram(NewModel(msgChan, retryChan), tea.WithContext(ctx))

	// Start tea
	wg.Add(1)
	var teaErr error
	go func() {
		_, err = t.Run()

		if !errors.Is(err, tea.ErrProgramKilled) && !errors.Is(err, tea.ErrInterrupted) {
			teaErr = err
		}

		// If context not yet cancelled
		if err := ctx.Err(); err == nil {
			cancel()
		}

		wg.Done()
	}()

	// Start ollama
	wg.Add(1)
	var ollamaErr error
	go func() {
		ollamaErr = Start(ctx, url, model, headers, options, msgChan, retryChan)

		// If context not yet cancelled
		if err := ctx.Err(); err == nil {
			cancel()
		}

		wg.Done()
	}()

	// Wait for both model & ollama
	wg.Wait()

	if teaErr != nil {
		printErr("%v", teaErr)
	}
	if ollamaErr != nil {
		printErr("%v", ollamaErr)
	}
}

func Start(ctx context.Context, url *url.URL, model string, headers map[string]string, options map[string]any, msgChan chan Msg, retryChan chan int) error {
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
		return err
	}

	// Get diff
	diffResp, err := diff()
	if err != nil {
		return err
	}
	if strings.TrimSpace(diffResp) == "" {
		return errors.New(`no changes added to commit (use "git add" and/or "git commit -a")`)
	}

	// Set keywords
	keywords := []string{"fix", "feat", "build", "chore", "ci", "docs", "style", "refactor", "perf", "test"}

	// Set prompts
	systemPrompt := fmt.Sprintf(`You are to act as an author of a commit message in git. 
				You will be given the output of the 'git diff --staged' command, and you are to convert it into a commit message. 
				Craft a concise, single sentence commit message that encapsulates all changes made, with an emphasis on the primary updates. 
				If the modifications share a common theme or scope, mention it succinctly; otherwise, leave the scope out to maintain focus. 
				The goal is to provide a clear and unified overview of the changes in one single message.
				Do not preface the commit with anything except for the conventional commit keywords: %s.`, strings.Join(keywords, ", "))
	prompt := fmt.Sprintf("%s\nHere is the output of 'git diff --staged': ```\n%s\n```", systemPrompt, diffResp)

	commitMsg := ""

retry:
	for {
		msgChan <- Msg{
			kind: MsgLoading,
		}

		thoughts := ""
		response := ""
		thinking := false
		startTag := regexp.MustCompile("<(.*)>")
		var endTag *regexp.Regexp

		// Generate
		err = ollama.Generate(ctx, &api.GenerateRequest{
			Model:   model,
			Prompt:  prompt,
			Options: options,
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
							msgChan <- Msg{
								text: split[1],
								kind: MsgThought,
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
						msgChan <- Msg{
							text: split[0],
							kind: MsgThought,
						}
						msgChan <- Msg{
							text: split[1],
							kind: MsgResponse,
						}

						return nil
					}

					msgChan <- Msg{
						text: gr.Response,
						kind: MsgThought,
					}

					return nil
				}

				// This is a response
				response += gr.Response
				msgChan <- Msg{
					text: gr.Response,
					kind: MsgResponse,
				}

				return nil
			},
		)
		if err != nil {
			return err
		}

		// Check if user quit
		if err := ctx.Err(); err != nil {
			return nil
		}

		// Extract commit message from response
		lines := strings.Split(response, "\n")

	extract:
		for i := range lines {
			line := lines[len(lines)-i-1] // backwards -> forwards

			for _, keyword := range keywords {
				if !strings.Contains(line, keyword) {
					continue
				}

				// Remove special characters
				response = strings.ReplaceAll(line, "`", "")
				response = strings.ReplaceAll(response, "*", "")
				break extract
			}
		}

		// Format response into commit
		commitMsg = strings.TrimSpace(response)
		if commitMsg == "" {
			return errors.New("no commit message found")
		}

		// Have user validate commit message
		msgChan <- Msg{
			text: commitMsg,
			kind: MsgDone,
		}

		select {

		// Check if user quit
		case <-ctx.Done():
			return nil

		// Check if user wants to retry
		case retry := <-retryChan:
			if retry == 0 {
				break retry
			}
		}
	}

	// Commit
	commitResp, err := commit(commitMsg)
	if err != nil {
		return err
	}
	msgChan <- Msg{
		kind: MsgThought,
		text: commitResp,
	}

	return nil
}

// git diff --staged
func diff() (string, error) {
	// Get diff
	cmd := exec.Command("git", "diff", "--staged")
	diff, err := cmd.Output()
	if err != nil {
		return "", errors.Join(err, errors.New(string(diff)))
	}

	// Remove unnecessary '+' prefix
	diffStr := ""
	for line := range strings.SplitSeq(strings.TrimSuffix(string(diff), "\n"), "\n") {
		diffStr += strings.TrimPrefix(line, "+") + "\n"
	}

	return diffStr, nil
}

// git commit -m
func commit(message string) (string, error) {
	cmd := exec.Command("git", "commit", "-m", message)
	gitCommit, err := cmd.Output()
	if err != nil {
		return "", errors.Join(err, errors.New(string(gitCommit)))
	}

	return string(gitCommit), nil
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
