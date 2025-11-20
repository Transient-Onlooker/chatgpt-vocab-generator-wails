package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// VocabApp struct
type VocabApp struct {
	ctx    context.Context
	client *openai.Client
}

// NewVocabApp creates a new App application struct
func NewVocabApp() *VocabApp {
	return &VocabApp{}
}

// --- Wails Lifecycle ---

// startup is called when the app starts. It's a good place to consume
// the context, and to initialize things.
func (a *VocabApp) startup(ctx context.Context) {
	a.ctx = ctx
	apiKey := loadAPIKey()
	if apiKey != "" {
		a.client = openai.NewClient(apiKey)
	} else {
		runtime.LogErrorf(a.ctx, "API 키를 찾을 수 없습니다. api.json 파일을 확인하세요.")
	}
}

// --- Structs & Helpers ---

type APIKeyConfig struct {
	APIKey string `json:"chatgpt_api_key"`
}

type VocabPair struct {
	Word   string
	Senses []string
}

// --- Go functions callable from Javascript ---

func (a *VocabApp) OpenFile() (string, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "단어장 TXT 파일 선택",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "텍스트 파일 (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if selection == "" {
		return "", fmt.Errorf("파일이 선택되지 않았습니다")
	}

	content, err := os.ReadFile(selection)
	if err != nil {
		return "", fmt.Errorf("파일 읽기 오류: %w", err)
	}

	return string(content), nil
}

func (a *VocabApp) SaveFile(contentToSave string, suggestedFilename string) (string, error) {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "결과 저장",
		DefaultFilename: suggestedFilename,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "텍스트 파일 (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", fmt.Errorf("저장 경로가 선택되지 않았습니다")
	}

	err = os.WriteFile(filePath, []byte(contentToSave), 0644)
	if err != nil {
		return "", fmt.Errorf("파일 저장 오류: %w", err)
	}
	return fmt.Sprintf("저장 완료: %s", filepath.Base(filePath)), nil
}

func (a *VocabApp) Generate(vocabBlock string, modelID string, questionType string, numSentences int) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("API 클라이언트가 초기화되지 않았습니다. API 키를 확인하세요.")
	}

	parsed := parseVocabBlock(vocabBlock)
	if len(parsed) == 0 {
		return "", fmt.Errorf("입력에서 유효한 'word = 뜻' 형식을 찾을 수 없습니다.")
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(parsed), func(i, j int) { parsed[i], parsed[j] = parsed[j], parsed[i] })

	systemPrompt, userPrompt := buildPrompts(parsed, questionType, numSentences)
	
	outputText, err := a.callChatGPT(modelID, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	return outputText, nil
}


// --- Internal Go Logic ---

func loadAPIKey() string {
	exePath, err := os.Executable()
	if err != nil {
		// Fallback for environments where Executable is not available
		exePath, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	exeDir := filepath.Dir(exePath)
	apiPath := filepath.Join(exeDir, "api.json")

	// Also check in the development path
	if _, err := os.Stat(apiPath); os.IsNotExist(err) {
		apiPath = "api.json" // Look in the current working dir for `wails dev`
	}
	
	file, err := os.ReadFile(apiPath)
	if err != nil {
		return ""
	}

	var config APIKeyConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		return ""
	}
	return config.APIKey
}


func parseVocabBlock(vocabBlock string) []VocabPair {
	var pairs []VocabPair
	re := regexp.MustCompile(`[;,]`)
	for _, raw := range strings.Split(vocabBlock, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}
		word := strings.TrimSpace(parts[0])
		meaningsRaw := strings.TrimSpace(parts[1])
		sensesRaw := re.Split(meaningsRaw, -1)
		var senses []string
		for _, s := range sensesRaw {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				senses = append(senses, trimmed)
			}
		}
		if word != "" && len(senses) > 0 {
			pairs = append(pairs, VocabPair{Word: word, Senses: senses})
		}
	}
	return pairs
}

func buildPrompts(parsed []VocabPair, questionType string, numSentences int) (string, string) {
	distributionRule := "2. CRITICAL: The position of the correct answer MUST be truly and unpredictably randomized to ensure a balanced distribution. For the entire set of questions, each choice position (①, ②, ③, ④, ⑤) should be the correct answer approximately 20% of the time. DO NOT use any discernible pattern (e.g., 1, 2, 3, 4, 5 or 5, 4, 3, 2, 1). The sequence of correct answers must appear random and chaotic."
	selfCorrectionRule := "### Final Review\nBefore concluding your response, you MUST review the entire generated text one last time to ensure every single rule has been followed. Pay special attention that every question has exactly 5 numbered choices (① to ⑤). If you find any mistake, you must correct it before finishing."

	var systemPrompt string
	switch questionType {
	case "빈칸 추론":
		systemPrompt = strings.Join([]string{
			"You are an expert English vocabulary test maker for Korean students.",
			"Your task is to create multiple-choice questions that test understanding of words in context.",
			"Strictly follow all rules below.",
			"",
			"### Main Rule",
			"For each WORD and for each of its SENSEs, you must generate a complete question block.",
			"",
			"### Word Selection & Question Style Rule",
			"1. PRIORITY: Focus on polysemous words—those with multiple, distinct meanings (e.g., different parts of speech like 'conduct' as a noun vs. verb, or different senses like 'bank' of a river vs. a financial institution).",
			"2. GOAL: The questions should be intentionally challenging, designed to confuse the test-taker and test their ability to discern the correct meaning from context.",
			"",
			"### Answer Generation Rules",
			"1. CRITICAL: DO NOT mark the correct answer in the choices. Instead, create a separate `[정답]` section at the very end of the entire output, listing each question number and its correct choice number.",
			distributionRule,
			"",
			"### Output Structure (per question)",
			"1. Start with the question number (e.g., '1.').",
			"2. Add the title: '다음 빈칸에 공통으로 들어갈 말로 가장 적절한 것은?'",
			fmt.Sprintf("3. Provide exactly %d distinct English sentences as context. Each sentence must have the word blanked out as '_______'.", numSentences),
			"4. Provide exactly 5 answer choices (①, ②, ③, ④, ⑤).",
			"5. The choices must include one correct answer (the original WORD) and four plausible but incorrect distractors.",
			"6. Separate each full question block with a '---' line.",
			"",
			selfCorrectionRule,
		}, "\n")
	case "영영풀이":
		systemPrompt = strings.Join([]string{
			"You are an expert English vocabulary test maker for Korean students.",
			"Your task is to create multiple-choice questions based on English definitions.",
			"Strictly follow all rules below.",
			"",
			"### Main Rule",
			"For each WORD, you must generate one complete multiple-choice question.",
			"",
			"### Word Selection & Question Style Rule",
			"1. PRIORITY: Focus on polysemous words—those with multiple, distinct meanings (e.g., different parts of speech like 'conduct' as a noun vs. verb, or different senses like 'bank' of a river vs. a financial institution).",
			"2. GOAL: The questions should be intentionally challenging, designed to confuse the test-taker and test their ability to discern the correct meaning from context.",
			"",
			"### Answer Generation Rules",
			"1. CRITICAL: DO NOT mark the correct answer in the choices. Instead, create a separate `[정답]` section at the very end of the entire output, listing each question number and its correct choice number.",
			distributionRule,
			"",
			"### Output Structure (per question)",
			"1. Start with the question number (e.g., '1.').",
			"2. Add the title: '다음 영어 설명에 해당하는 단어는?'",
			"3. Provide the English definition of the WORD as the question body.",
			"4. Provide exactly 5 answer choices (①, ②, ③, ④, ⑤): one correct answer (the original WORD) and four plausible distractors (e.g., synonyms, related words).",
			"5. Separate each full question block with a '---' line.",
			"",
			selfCorrectionRule,
		}, "\n")
	case "뜻풀이 판단":
		systemPrompt = strings.Join([]string{
			"You are an expert English vocabulary test maker for Korean students.",
			"Your task is to create multiple-choice questions that test the precise definition of a word.",
			"Strictly follow all rules below.",
			"",
			"### Main Rule",
			"For each WORD, you must generate one complete multiple-choice question asking for its correct definition.",
			"",
			"### Word Selection & Question Style Rule",
			"1. PRIORITY: Focus on polysemous words—those with multiple, distinct meanings (e.g., different parts of speech like 'conduct' as a noun vs. verb, or different senses like 'bank' of a river vs. a financial institution).",
			"2. GOAL: The questions should be intentionally challenging, designed to confuse the test-taker and test their ability to discern the correct meaning from context.",
			"",
			"### Answer Generation Rules",
			"1. CRITICAL: DO NOT mark the correct answer in the choices. Instead, create a separate `[정답]` section at the very end of the entire output, listing each question number and its correct choice number.",
			distributionRule,
			"",
			"### Output Structure (per question)",
			"1. Start with the question number (e.g., '1.').",
			"2. Add the title: '다음 단어 <WORD>의 영영풀이로 가장 적절한 것은?' (replace <WORD> with the actual word).",
			"3. Provide exactly 5 definition choices (①, ②, ③, ④, ⑤): one perfectly correct definition and four subtly incorrect but plausible definitions.",
			"4. Separate each full question block with a '---' line.",
			"",
			selfCorrectionRule,
		}, "\n")
	}

	var parsedForModelText []string
	for _, pair := range parsed {
		parsedForModelText = append(parsedForModelText, fmt.Sprintf("%s = %s", pair.Word, strings.Join(pair.Senses, ", ")))
	}

	userPrompt := strings.Join([]string{
		"Here is the list of vocabulary. Create test questions based on these words, strictly following all rules defined in the system instructions.",
		"",
		"[Vocabulary List]",
		strings.Join(parsedForModelText, "\n"),
	}, "\n")

	return systemPrompt, userPrompt
}

func (a *VocabApp) callChatGPT(model string, systemPrompt string, userPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	resp, err := a.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
			Temperature: 1.0,
		},
	)

	if err != nil {
		return "", fmt.Errorf("ChatGPT API 오류: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("API가 빈 텍스트를 반환했습니다")
	}

	return resp.Choices[0].Message.Content, nil
}