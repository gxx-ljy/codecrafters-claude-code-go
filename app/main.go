package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"golang.org/x/text/message"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))
	messages := []openai.ChatCompletionMessageParamUnion{
		{
			OfUser: &openai.ChatCompletionUserMessageParam{
				Content: openai.ChatCompletionUserMessageParamContentUnion{
					OfString: openai.String(prompt),
				},
			},
		},
	}

	// 定义tools
	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "Read",
			Description: openai.String("Read and return the contents of a file"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"file_path"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "Write",
			Description: openai.String("Write a file"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path of the file to write to",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The content to write to the file",
					},
				},
				"required": []string{"file_path", "content"},
			},
		}),
	}
	for {
		resp, err := client.Chat.Completions.New(context.Background(),
			openai.ChatCompletionNewParams{
				Model: "anthropic/claude-haiku-4.5",
				Messages: messages,
				Tools: tools,
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(resp.Choices) == 0 {
			panic("No choices in response")
		}

		// You can use print statements as follows for debugging, they'll be visible when running tests.
		fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

		// 处理tool calls
		if len(resp.Choices[0].Message.ToolCalls) > 0 {
			choice := resp.Choices[0]
			messages = append(messages, choice.Message.ToParam())

			for _, toolCall := range resp.Choices[0].Message.ToolCalls {
				// 解析函数参数
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing tool arguments: %v\n", err)
					continue
				}

				if toolCall.Function.Name == "Read" {
					// 获取file_path参数
					filePath, ok := args["file_path"].(string)
					if !ok {
						fmt.Fprintln(os.Stderr, "file_path argument is not a string")
						continue
					}

					// 读取文件并输出内容
					content, err := os.ReadFile(filePath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
						continue
					}
					// fmt.Print(string(content))
					// 将工具响应添加到消息中
					messages = append(messages, openai.ChatCompletionMessageParamUnion{
						OfTool: &openai.ChatCompletionToolMessageParam{
							ToolCallID: toolCall.ID,
							Content:    openai.ChatCompletionToolMessageParamContentUnion{
								OfString: openai.String(string(content)),
							},
						},
					})
				}
				if toolCall.Function.Name == "Write" {
					// 获取file_path参数
					filePath, ok := args["file_path"].(string)
					if !ok {
						fmt.Fprintln(os.Stderr, "file_path argument is not a string")
						continue
					}

					// 获取工具响应内容
					content, ok := choice.Message.ToolCalls[0].Output.Content.(string)
					if !ok {
						fmt.Fprintln(os.Stderr, "tool output content is not a string")
						continue
					}
					// 写入文件
					err := os.WriteFile(filePath, []byte(content), 0644)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error writing file: %v\n", err)
						continue
					}

					messages = append(messages, openai.ChatCompletionMessageParamUnion{
						OfTool: &openai.ChatCompletionToolMessageParam{
							ToolCallID: toolCall.ID,
							Content:    openai.ChatCompletionToolMessageParamContentUnion{
								OfString: openai.String("OK"),
							},
						},
					})
				}
			}
		} else {
			// 如果没有tool calls，直接输出内容
			fmt.Print(resp.Choices[0].Message.Content)
			return
		}
	}
}