
ファイルを一括でshiftjisからutf8に変換

### 1. **files** - 個別ファイル指定

```bash
# 単一ファイル
sj2u files job/tmp/sample_01.log

# 複数ファイル（カンマ区切り）
sj2u files job/tmp/sample_01.log,job/tmp/test*.log

# Globパターン
sj2u files data/*.csv,logs/test*.log
```

### 2. **dir** - ディレクトリ一括変換

```bash
# 基本
sj2u dir ./test/tmp

# 深さ指定
sj2u dir --depth 2 ./test/tmp 

# パターン指定
sj2u dir --depth 2 --patterns "*.md,*.log" ./test/tmp
```

### 3. **clear** - 履歴クリア

```bash
sj2u clear
```

## 📝 YAML設定ファイル例

### files
```yaml
mode: files
files:
  - job/tmp/sample_01.log
  - "job/tmp/test*.log"
  - "data/*.csv"
```

### dir
```yaml
mode: dir
dir: ./test/tmp
depth: 2
patterns:
  - "*.md"
  - "*.log"
```

### clear
```yaml
mode: clear
```
