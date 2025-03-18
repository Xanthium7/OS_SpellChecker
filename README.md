# Go Spell Checker

A fast and accurate spell checking application built with Go. This application monitors your clipboard and allows you to correct spelling mistakes with a simple hotkey (Ctrl+Alt+S).

![Spell Checker Demo](https://github.com/user-attachments/assets/163d6662-d0e4-4657-8cc3-ed69645142ed)

## Features

- **System tray application** - Always available but stays out of your way
- **Clipboard integration** - Reads and writes to your clipboard
- **Hotkey support** - Quick access with Ctrl+Alt+S
- **Fast spell checking** - Uses the Trie data structure for efficient word lookups
- **Smart correction** - Uses edit distance and word frequency to find the best corrections

## How It Works

### Architecture Overview

The application consists of several key components:

1. **Dictionary Management** - Stores words in a Trie data structure
2. **Spell Checking Algorithm** - Uses edit distance to find corrections
3. **Clipboard Integration** - Reads and writes text
4. **System Tray UI** - Provides easy access to functionality

### The Trie Data Structure

A Trie (pronounced "try") is a tree-like data structure perfect for storing and retrieving strings. It's especially efficient for spell checking.

```go
type TrieNode struct {
    children map[rune]*TrieNode
    isEnd    bool
}

type Trie struct {
    root *TrieNode
}
```

**How the Trie works:**

- Each node represents a character in a word
- The path from the root to any node forms a prefix
- Nodes marked with `isEnd = true` represent complete words
- Searching takes O(m) time, where m is the length of the word being searched

**Inserting words:**

```go
func (t *Trie) insert(word string) {
    node := t.root
    for _, ch := range word {
        if _, exists := node.children[ch]; !exists {
            node.children[ch] = newTrieNode()
        }
        node = node.children[ch]
    }
    node.isEnd = true
}
```

This traverses the trie character by character, creating new nodes as needed, and marks the final node as a word ending.

**Searching for words:**

```go
func (t *Trie) search(word string) bool {
    node := t.root
    for _, ch := range word {
        if _, exists := node.children[ch]; !exists {
            return false
        }
        node = node.children[ch]
    }
    return node.isEnd
}
```

This traverses the trie character by character, returning false if any character isn't found, and checking if the final node represents a complete word.

### Spell Checking Algorithm

The spell checking process consists of several steps:

1. **Text Parsing**
   ```go
   words := strings.FieldsFunc(text, func(r rune) bool {
       return unicode.IsSpace(r) || unicode.IsPunct(r) && r != '\''
   })
   ```
   This splits text into words while preserving apostrophes.

2. **Word Processing**
   ```go
   // Extract prefix/suffix punctuation
   // Check capitalization
   // Skip short words or numbers
   ```
   Each word is analyzed to preserve its formatting and determine if it needs correction.

3. **Word Lookup**
   ```go
   if dictionary.search(lowerWord) {
       // Word exists, no correction needed
   }
   ```
   First, we check if the word exists in our dictionary.

4. **Finding Corrections**
   If a word isn't in the dictionary, we use the `findClosestMatch` function which:
   - Generates candidates with edit distance 1
   - If not enough candidates, tries edit distance 2
   - Scores candidates based on:
     - Edit distance (fewer changes = better)
     - Word frequency (common words prioritized)
     - Word length (similar length to original preferred)

5. **Candidate Generation**
   ```go
   func findCandidatesWithDistance(word string, maxDistance int) []Candidate {
       // Try deletions, transpositions, substitutions, insertions
   }
   ```
   This function implements the Damerau-Levenshtein distance algorithm, considering:
   - **Deletions**: Removing a character (hello → hell)
   - **Insertions**: Adding a character (helo → hello)
   - **Substitutions**: Replacing a character (hallo → hello)
   - **Transpositions**: Swapping adjacent characters (hlelo → hello)

6. **Candidate Scoring**
   ```go
   func getWordScore(word string, originalLen int) int {
       // Base score from frequency dictionary
       // Adjust for length similarity
       // Boost for common words
   }
   ```
   This assigns a quality score to each candidate to find the best match.

### Clipboard Integration

The application uses Windows API calls to interact with the clipboard:

1. **Reading from Clipboard**
   ```go
   func getClipboardText() string {
       // Open clipboard
       // Get data handle
       // Lock global memory
       // Convert to string
   }
   ```

2. **Writing to Clipboard**
   ```go
   func setClipboardText(text string) {
       // Open clipboard
       // Empty clipboard
       // Allocate memory
       // Copy data
       // Set clipboard data
   }
   ```

### System Tray Integration

The application uses the `systray` package to create a system tray icon and menu:

```go
func onReady() {
    systray.SetIcon(getIcon())
    systray.SetTitle("Spell Checker")
    systray.SetTooltip("Press Ctrl+Alt+S or click here to check spelling")
    mSpellCheck := systray.AddMenuItem("Check Clipboard Spelling (Ctrl+Alt+S)", "Check spelling of clipboard text")
    mQuit := systray.AddMenuItem("Quit", "Quit the application")
    
    // Handle menu events
}
```

### Hotkey Registration

The application registers a global hotkey (Ctrl+Alt+S) to trigger spell checking:

```go
go func() {
    registerHotKey.Call(0, 1, MOD_CTRL|MOD_ALT, VK_S)
    var msg win.MSG
    for {
        if ret, _, _ := getMessageA.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0); ret != 0 {
            if msg.Message == win.WM_HOTKEY {
                checkSpelling()
            }
        }
    }
}()
```

## Example Usage

### Original Text (with typos):
```
Ths is a tst sentnce with sme typose to chck the spel cheker.
```

### Corrected Text:
```
This is a test sentence with some typos to check the spell checker.
```

### How the Corrections Work:

| Misspelled Word | Corrected Word | How It's Found |
|-----------------|----------------|----------------|
| Ths | This | Edit distance 1: Insert 'i' |
| tst | test | Edit distance 1: Insert 'e' |
| sentnce | sentence | Edit distance 1: Insert 'e' |
| sme | some | Edit distance 1: Insert 'o' |
| typose | typos | Edit distance 1: Delete 'e' |
| chck | check | Edit distance 1: Insert 'e' |
| spel | spell | Edit distance 1: Insert 'l' |
| cheker | checker | Edit distance 1: Insert 'c' |

## Word Score Calculation

When multiple potential corrections exist, the algorithm chooses the best one using several factors:

```go
func getWordScore(word string, originalLen int) int {
    // Base score from frequency dictionary
    score := WordFrequency[word]
    
    // Adjust score based on length similarity
    lenDiff := abs(len(word) - originalLen)
    if lenDiff == 0 {
        score += 100 // Bonus for same length
    } else {
        score -= lenDiff * 10 // Penalty for different length
    }
    
    // Prefer shorter words when score is similar
    score -= len(word)
    
    // Common words get a boost
    if commonWords[word] {
        score += 200
    }
    
    return score
}
```

This scoring system prioritizes:
1. Words that appear frequently in the dictionary
2. Words that are the same length as the original misspelled word
3. Common everyday words
4. Shorter words when all else is equal

## Performance Considerations

The spell checker uses several optimizations:

1. **Limiting edit distance**: Only considers words up to 2 edits away
2. **Early candidate limiting**: Stops searching after finding enough good candidates
3. **Smart dictionary**: Common words are given priority to improve accuracy
4. **Skipping short words**: Words with 1-2 characters are typically not corrected

## Dictionary Management

The application loads words from a dictionary file (`dictionary.txt`) at startup, but also has a built-in fallback dictionary with common English words:

```go
func loadBuiltInDictionary() {
    commonWords := []string{
        "the", "is", "a", "an", "and", "are", "as", "at", "be", "but", "by", 
        // ...many more common words...
    }
    
    weight := 5000 // High priority for common words
    for _, word := range commonWords {
        if !dictionary.search(word) {
            dictionary.insert(word)
            WordFrequency[word] = weight
            weight--
        }
    }
}
```

## Areas for Improvement

1. **Context-aware corrections**: Consider surrounding words for better corrections
2. **Learning capabilities**: Improve suggestions based on user choices
3. **Domain-specific dictionaries**: Support for specialized terminology
4. **Language detection**: Auto-detect and apply the appropriate dictionary
5. **N-gram models**: Use statistical models for better candidate ranking

## Conclusion

This Go Spell Checker demonstrates how data structures like Tries and algorithms like edit distance can be combined to create a practical and efficient utility. The careful balance of accuracy, performance, and usability make it suitable for real-world use.

