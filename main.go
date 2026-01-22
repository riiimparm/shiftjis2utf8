package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"gopkg.in/yaml.v3"
)

const (
	historyFile = ".local.shiftjis2utf8"
	configFile  = ".shiftjis2utf8.yaml"
)

type Config struct {
	Mode     string   `yaml:"mode"`
	Files    []string `yaml:"files"`
	Dir      string   `yaml:"dir"`
	Patterns []string `yaml:"patterns"`
	Depth    int      `yaml:"depth"`
}

type ConversionHistory struct {
	converted map[string]bool
	path      string
}

func main() {
	// 引数なしの場合は設定ファイルをチェック
	if len(os.Args) < 2 {
		config, configExists := loadConfig(configFile)
		if configExists {
			// YAML設定で実行
			executeFromConfig(config)
			return
		}
		showUsage()
		os.Exit(0)
	}

	mode := os.Args[1]

	// clearモードは即座に実行
	if mode == "clear" {
		clearHistory()
		return
	}

	// filesモード
	if mode == "files" {
		filesCmd := flag.NewFlagSet("files", flag.ExitOnError)
		filesCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, `使い方: %s files <ファイルパス...>

引数:
  <ファイルパス>    カンマ区切りでファイルパスを指定（Globパターン可）
                    例: sample.csv,test*.log


例:
  %s files job/tmp/sample_01.log,job/tmp/test*.log
  %s files data.csv,output/*.txt

`, os.Args[0], os.Args[0], os.Args[0])
		}

		filesCmd.Parse(os.Args[2:])

		for _, arg := range filesCmd.Args() {
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "エラー: オプション '%s' はファイルパスの前に指定する必要があります。\n", arg)
				fmt.Fprintf(os.Stderr, "正しい例: %s files --patterns \"*.ini\" job/tmp/sample_01.log\n", os.Args[0])
				os.Exit(1)
			}
		}

		if filesCmd.NArg() < 1 {
			filesCmd.Usage()
			os.Exit(1)
		}

		filePatterns := strings.Split(filesCmd.Arg(0), ",")
		runFilesMode(filePatterns)
		return
	}

	// dirモード
	if mode == "dir" {
		dirCmd := flag.NewFlagSet("dir", flag.ExitOnError)
		depth := dirCmd.Int("depth", 1, "再帰探索の深さ")
		patterns := dirCmd.String("patterns", "*.txt", "対象ファイルパターン（カンマ区切り）")
		dirCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, `使い方: %s dir <ディレクトリパス> [オプション]

引数:
  <ディレクトリパス>    検索対象ディレクトリ

オプション:
  --depth <n>          再帰探索の深さ（デフォルト: 1）
  --patterns <pattern> 対象ファイルパターン（デフォルト: *.txt）

例:
  %s dir ./test/tmp --depth 2 --patterns "*.md,*.log"
  %s dir ./data --patterns "*.csv"

`, os.Args[0], os.Args[0], os.Args[0])
		}

		dirCmd.Parse(os.Args[2:])

		for _, arg := range dirCmd.Args() {
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "エラー: オプション '%s' はディレクトリパスの前に指定する必要があります。\n", arg)
				fmt.Fprintf(os.Stderr, "正しい例: %s dir --patterns \"*.ini\" job/tmp\n", os.Args[0])
				os.Exit(1)
			}
		}

		if dirCmd.NArg() < 1 {
			dirCmd.Usage()
			os.Exit(1)
		}

		targetDir := dirCmd.Arg(0)
		patternList := strings.Split(*patterns, ",")
		runDirMode(targetDir, patternList, *depth)
		return
	}

	// モード不明
	fmt.Fprintf(os.Stderr, " 不明なモード: %s\n\n", mode)
	showUsage()
	os.Exit(1)
}

func executeFromConfig(config Config) {
	switch config.Mode {
	case "files":
		if len(config.Files) == 0 {
			fmt.Fprintf(os.Stderr, " filesモードですが、filesが指定されていません\n")
			os.Exit(1)
		}
		runFilesMode(config.Files)
	case "dir":
		if config.Dir == "" {
			config.Dir = "."
		}
		if len(config.Patterns) == 0 {
			config.Patterns = []string{"*.txt"}
		}
		if config.Depth == 0 {
			config.Depth = 1
		}
		runDirMode(config.Dir, config.Patterns, config.Depth)
	case "clear":
		clearHistory()
	default:
		fmt.Fprintf(os.Stderr, " 不明なモード: %s\n", config.Mode)
		showUsage()
		os.Exit(1)
	}
}

func showUsage() {
	fmt.Fprintf(os.Stderr, `使い方: %s <モード> [引数...] [オプション...]

モード:
  files <ファイルパス>        指定したファイルを変換
  dir <ディレクトリパス>      ディレクトリ内のファイルを一括変換
  clear                      変換履歴をクリア

詳細なヘルプ:
  %s files -h
  %s dir -h

設定ファイル (.shiftjis2utf8.yaml):
  mode: files
  files:
    - job/tmp/sample_01.log
    - "test*.log"

  または

  mode: dir
  dir: ./test/tmp
  depth: 2
  patterns:
    - "*.md"
    - "*.log"

例:
  %s files job/tmp/sample_01.log,job/tmp/test*.log
  %s dir ./test/tmp --depth 2 --patterns "*.md,*.log"
  %s clear

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func clearHistory() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("ホームディレクトリの取得に失敗: %v", err)
	}
	historyPath := filepath.Join(homeDir, historyFile)

	if err := os.Remove(historyPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("履歴ファイルの削除に失敗: %v", err)
	}
	fmt.Println("変換履歴をクリアしました")
}

func runFilesMode(filePatterns []string) {

	history := getHistory()

	fmt.Printf("ファイルモード\n")
	fmt.Printf("対象: %v\n", filePatterns)
	fmt.Println("---------------------------------")

	convertedCount := 0
	skippedCount := 0

	for _, pattern := range filePatterns {
		pattern = strings.TrimSpace(pattern)

		// Globパターン展開
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			// パターンが展開できない場合は直接ファイルとして処理
			matches = []string{pattern}
		}

		for _, filePath := range matches {
			if processFile(filePath, history) {
				convertedCount++
			} else {
				skippedCount++
			}
		}
	}

	if err := history.save(); err != nil {
		log.Printf("履歴の保存に失敗: %v", err)
	}

	fmt.Println("---------------------------------")
	fmt.Printf("処理完了！\n")
	fmt.Printf("変換: %d個 | スキップ: %d個\n", convertedCount, skippedCount)
}

func runDirMode(targetDir string, patterns []string, depth int) {

	history := getHistory()

	fmt.Printf("ディレクトリモード\n")
	fmt.Printf("対象ディレクトリ: %s\n", targetDir)
	fmt.Printf("探索深さ: %d\n", depth)
	fmt.Printf("対象パターン: %v\n", patterns)
	fmt.Println("---------------------------------")

	convertedCount := 0
	skippedCount := 0

	// パターンごとにファイル収集
	var targetFiles []string
	for _, pattern := range patterns {
		matches := findFilesByPattern(targetDir, strings.TrimSpace(pattern), depth)
		targetFiles = append(targetFiles, matches...)
	}

	// 重複削除
	uniqueFiles := make(map[string]bool)
	for _, f := range targetFiles {
		uniqueFiles[f] = true
	}

	for filePath := range uniqueFiles {
		if processFile(filePath, history) {
			convertedCount++
		} else {
			skippedCount++
		}
	}

	if err := history.save(); err != nil {
		log.Printf("履歴の保存に失敗: %v", err)
	}

	fmt.Println("---------------------------------")
	fmt.Printf("処理完了！\n")
	fmt.Printf("変換: %d個 | スキップ: %d個\n", convertedCount, skippedCount)
}

func processFile(filePath string, history *ConversionHistory) bool {
	absPath, _ := filepath.Abs(filePath)

	// ファイル存在チェック
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("ファイルが見つかりません: %s\n", filePath)
		return false
	}

	// 履歴チェック
	if history.isConverted(absPath) {
		return false
	}

	// UTF-8判定
	isUTF8, err := isFileUTF8(filePath)
	if err != nil {
		log.Printf("判定失敗: %s - %v\n", filepath.Base(filePath), err)
		return false
	}
	if isUTF8 {
		fmt.Printf("既にUTF-8: %s\n", filepath.Base(filePath))
		history.markConverted(absPath)
		return false
	}

	// Shift-JIS → UTF-8 変換
	err = convertFileToUTF8(filePath)
	if err != nil {
		log.Printf("変換失敗: %s - %v\n", filepath.Base(filePath), err)
		return false
	}
	fmt.Printf("変換成功: %s\n", filepath.Base(filePath))
	history.markConverted(absPath)
	return true
}

func getHistory() *ConversionHistory {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("ホームディレクトリの取得に失敗: %v", err)
	}
	historyPath := filepath.Join(homeDir, historyFile)
	return loadHistory(historyPath)
}

// 設定ファイル読み込み
func loadConfig(path string) (Config, bool) {
	config := Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return config, false
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("設定ファイルの解析に失敗: %v", err)
		return config, false
	}

	return config, true
}

// 履歴読み込み
func loadHistory(path string) *ConversionHistory {
	history := &ConversionHistory{
		converted: make(map[string]bool),
		path:      path,
	}

	file, err := os.Open(path)
	if err != nil {
		return history
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		history.converted[scanner.Text()] = true
	}

	return history
}

// 履歴保存
func (h *ConversionHistory) save() error {
	file, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for path := range h.converted {
		fmt.Fprintln(writer, path)
	}
	return writer.Flush()
}

// 変換済みチェック
func (h *ConversionHistory) isConverted(path string) bool {
	return h.converted[path]
}

// 変換済みマーク
func (h *ConversionHistory) markConverted(path string) {
	h.converted[path] = true
}

// Globパターンでファイル検索
func findFilesByPattern(dir, pattern string, depth int) []string {
	var matches []string

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 深さ制限
		rel, _ := filepath.Rel(dir, path)
		if strings.Count(rel, string(os.PathSeparator)) >= depth {
			if info.IsDir() {
				return filepath.SkipDir
			}
		}

		if info.IsDir() {
			return nil
		}

		// パターンマッチ
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	return matches
}

// UTF-8判定
func isFileUTF8(filePath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	return utf8.Valid(buf[:n]), nil
}

// Shift-JIS → UTF-8 変換
func convertFileToUTF8(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ファイルを開けませんでした: %w", err)
	}
	defer file.Close()

	reader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
	utf8Bytes, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("文字コードの変換に失敗しました: %w", err)
	}

	err = os.WriteFile(filePath, utf8Bytes, 0644)
	if err != nil {
		return fmt.Errorf("ファイルの書き込みに失敗しました: %w", err)
	}

	return nil
}
