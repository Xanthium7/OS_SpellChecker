package main

import (
	"bufio"
	"log"
	"os"
	"sort"
	"strings"
	"syscall"
	"unicode"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/lxn/win"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	getClipboardData = user32.NewProc("GetClipboardData")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	setClipboardData = user32.NewProc("SetClipboardData")
	registerHotKey   = user32.NewProc("RegisterHotKey")
	getMessageA      = user32.NewProc("GetMessageA")
)

const (
	MOD_ALT  = 0x0001
	MOD_CTRL = 0x0002
	VK_S     = 0x53 // Virtual key code for 'S'

	// Maximum candidates to consider to avoid performance issues
	MAX_CANDIDATES = 10
	// Maximum distance to consider for corrections
	MAX_EDIT_DISTANCE = 2
)

// TrieNode represents a node in the Trie
type TrieNode struct {
	children map[rune]*TrieNode
	isEnd    bool
}

// Trie represents the trie data structure
type Trie struct {
	root *TrieNode
}

var dictionary *Trie

func newTrieNode() *TrieNode {
	return &TrieNode{
		children: make(map[rune]*TrieNode),
		isEnd:    false,
	}
}

func newTrie() *Trie {
	return &Trie{root: newTrieNode()}
}

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

// WordFrequency stores word frequency information
var WordFrequency map[string]int

func loadDictionary(filePath string) {
	dictionary = newTrie()
	WordFrequency = make(map[string]int)

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open dictionary file %s: %v", filePath, err)
		// Try to use a built-in fallback dictionary if the file is not found
		loadBuiltInDictionary()
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	wordCount := 0
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			word = strings.ToLower(word)
			dictionary.insert(word)
			WordFrequency[word] = 1000 - wordCount // Higher frequency for words at the start of the dictionary
			wordCount++
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading dictionary file: %v", err)
	}

	// If dictionary is too small, add built-in common words
	if wordCount < 100 {
		log.Printf("Dictionary loaded only %d words, adding built-in dictionary", wordCount)
		loadBuiltInDictionary()
	}

	log.Printf("Loaded %d words into the dictionary", len(WordFrequency))
}

// Load built-in dictionary with common English words
func loadBuiltInDictionary() {
	commonWords := []string{
		"the", "is", "a", "an", "and", "are", "as", "at", "be", "but", "by",
		"for", "if", "in", "into", "it", "no", "not", "of", "on", "or", "such",
		"that", "their", "then", "there", "these", "they", "this", "to", "was",
		"will", "with", "he", "she", "they", "them", "we", "us", "our", "you",
		"your", "him", "his", "her", "its", "my", "me", "mine", "sentence",
		"typos", "check", "spell", "checker", "some", "test", "have", "has", "had",
		"do", "does", "did", "can", "could", "would", "should", "may", "might",
		"must", "shall", "will", "from", "about", "like", "know", "think", "see",
		"come", "go", "get", "make", "say", "take", "find", "give", "tell", "work",
		"call", "try", "ask", "need", "feel", "become", "leave", "put", "mean", "keep",
		"let", "begin", "seem", "help", "talk", "turn", "start", "show", "hear", "play",
		"run", "move", "live", "happen", "stand", "lose", "pay", "meet", "include", "continue",
		"set", "learn", "change", "lead", "understand", "watch", "follow", "stop", "create",
		"speak", "read", "allow", "add", "spend", "grow", "open", "walk", "win", "offer",
		"remember", "appear", "buy", "wait", "serve", "die", "send", "expect", "build", "stay",
		"fall", "cut", "reach", "kill", "remain",
	}

	weight := 5000 // Give high priority to these common words
	for _, word := range commonWords {
		if !dictionary.search(word) {
			dictionary.insert(word)
			WordFrequency[word] = weight
			weight-- // Slightly reduce weight for each subsequent word
		}
	}
}

func main() {
	// Configure logging for better debugging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting spell checker application")

	// Load the dictionary
	loadDictionary("dictionary.txt")

	// Register hotkey (Ctrl+Alt+S)
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

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(getIcon())
	systray.SetTitle("Spell Checker")
	systray.SetTooltip("Press Ctrl+Alt+S or click here to check spelling")
	mSpellCheck := systray.AddMenuItem("Check Clipboard Spelling (Ctrl+Alt+S)", "Check spelling of clipboard text")
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	go func() {
		for {
			select {
			case <-mSpellCheck.ClickedCh:
				checkSpelling()
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func onExit() {
	// Cleanup
}

func checkSpelling() {
	text := getClipboardText()
	if text == "" {
		return
	}
	correctedText := correctSpelling(text)
	setClipboardText(correctedText)
}

// Candidate represents a potential spelling correction with its distance from the original word
type Candidate struct {
	word     string
	distance int
	score    int // Higher score = better match
}

func correctSpelling(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) && r != '\''
	})

	if len(words) == 0 {
		return text
	}

	// Process the text to maintain structure
	var result strings.Builder
	lastPos := 0

	for _, word := range words {
		// Find word position in original text
		wordPos := strings.Index(text[lastPos:], word)
		if wordPos == -1 {
			continue
		}
		wordPos += lastPos

		// Append text before the word
		result.WriteString(text[lastPos:wordPos])

		// Get the word with any surrounding punctuation
		prefix := ""
		suffix := ""
		cleanWord := word

		// Extract prefix punctuation
		for i := 0; i < len(cleanWord); i++ {
			if !unicode.IsLetter(rune(cleanWord[i])) && !unicode.IsNumber(rune(cleanWord[i])) {
				prefix += string(cleanWord[i])
			} else {
				cleanWord = cleanWord[i:]
				break
			}
		}

		// Extract suffix punctuation
		for i := len(cleanWord) - 1; i >= 0; i-- {
			if !unicode.IsLetter(rune(cleanWord[i])) && !unicode.IsNumber(rune(cleanWord[i])) && cleanWord[i] != '\'' {
				suffix = string(cleanWord[i]) + suffix
				cleanWord = cleanWord[:i]
			} else {
				break
			}
		}

		// Skip correcting if the word is empty, a number, or very short (1-2 chars)
		if len(cleanWord) <= 2 || isNumber(cleanWord) {
			result.WriteString(word)
			lastPos = wordPos + len(word)
			continue
		}

		// Check if word needs correction
		isCapitalized := unicode.IsUpper(rune(cleanWord[0]))
		isAllCaps := isAllUppercase(cleanWord)

		lowerWord := strings.ToLower(cleanWord)

		// Skip correction if word is in dictionary
		if dictionary.search(lowerWord) {
			result.WriteString(word)
			lastPos = wordPos + len(word)
			continue
		}

		// Find correction
		corrected := findClosestMatch(lowerWord)

		// Apply original capitalization
		if corrected != lowerWord {
			if isAllCaps {
				corrected = strings.ToUpper(corrected)
			} else if isCapitalized {
				corrected = strings.ToUpper(string(corrected[0])) + corrected[1:]
			}
		} else {
			// No correction found, use original
			corrected = cleanWord
		}

		result.WriteString(prefix + corrected + suffix)
		lastPos = wordPos + len(word)
	}

	// Append any remaining text
	result.WriteString(text[lastPos:])
	return result.String()
}

func isNumber(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func isAllUppercase(s string) bool {
	for _, c := range s {
		if unicode.IsLetter(c) && !unicode.IsUpper(c) {
			return false
		}
	}
	return true
}

func findClosestMatch(word string) string {
	log.Printf("Finding closest match for: %s", word)

	if dictionary.search(word) {
		log.Printf("Word '%s' found in dictionary", word)
		return word
	}

	// Generate candidates with edit distances 1 and 2
	candidates := []Candidate{}

	// Try edit distance 1 first
	candidates = append(candidates, findCandidatesWithDistance(word, 1)...)

	// If we don't have enough good candidates, try edit distance 2
	if len(candidates) < 3 {
		candidates = append(candidates, findCandidatesWithDistance(word, 2)...)
	}

	if len(candidates) == 0 {
		log.Printf("No match found for '%s'", word)
		return word
	}

	// Sort candidates by score (higher is better)
	sort.Slice(candidates, func(i, j int) bool {
		// First priority: lower edit distance
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}

		// Second priority: word frequency/score
		return candidates[i].score > candidates[j].score
	})

	bestCandidate := candidates[0].word
	log.Printf("Corrected '%s' to '%s' (score: %d)", word, bestCandidate, candidates[0].score)
	return bestCandidate
}

func findCandidatesWithDistance(word string, maxDistance int) []Candidate {
	candidates := []Candidate{}
	wordLen := len(word)

	// 1. Try deletions (edit distance 1)
	for i := 0; i < wordLen; i++ {
		deletion := word[:i] + word[i+1:]
		if dictionary.search(deletion) {
			score := getWordScore(deletion, wordLen-1)
			candidates = append(candidates, Candidate{deletion, 1, score})
		}
	}

	// 2. Try transpositions (edit distance 1)
	for i := 0; i < wordLen-1; i++ {
		transposition := word[:i] + string(word[i+1]) + string(word[i]) + word[i+2:]
		if dictionary.search(transposition) {
			score := getWordScore(transposition, wordLen)
			candidates = append(candidates, Candidate{transposition, 1, score})
		}
	}

	// 3. Try substitutions (edit distance 1)
	for i := 0; i < wordLen; i++ {
		for c := 'a'; c <= 'z'; c++ {
			if c != rune(word[i]) {
				substitution := word[:i] + string(c) + word[i+1:]
				if dictionary.search(substitution) {
					score := getWordScore(substitution, wordLen)
					candidates = append(candidates, Candidate{substitution, 1, score})
				}
			}
		}
	}

	// 4. Try insertions (edit distance 1)
	for i := 0; i <= wordLen; i++ {
		for c := 'a'; c <= 'z'; c++ {
			insertion := word[:i] + string(c) + word[i:]
			if dictionary.search(insertion) {
				score := getWordScore(insertion, wordLen+1)
				candidates = append(candidates, Candidate{insertion, 1, score})
			}
		}
	}

	// Limit candidates before trying edit distance 2
	if len(candidates) > MAX_CANDIDATES/2 {
		// Sort by score and take the top half
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})
		candidates = candidates[:MAX_CANDIDATES/2]
		return candidates
	}

	// If we need edit distance 2, we do a second round of edits
	if maxDistance >= 2 {
		// For each potential edit1 word (whether in dictionary or not)
		edits1 := generateAllEdits1(word)

		for _, edit1 := range edits1 {
			// Skip if this is already a candidate
			if containsWord(candidates, edit1) {
				continue
			}

			// Try another set of edits on this edit1 word
			for i := 0; i < len(edit1); i++ {
				// Deletions
				if i < len(edit1) {
					edit2 := edit1[:i] + edit1[i+1:]
					if dictionary.search(edit2) && !containsWord(candidates, edit2) {
						score := getWordScore(edit2, len(edit2))
						candidates = append(candidates, Candidate{edit2, 2, score})
						if len(candidates) >= MAX_CANDIDATES {
							return candidates
						}
					}
				}

				// Substitutions (limited to improve performance)
				if i < len(edit1) {
					for c := 'a'; c <= 'z'; c += 2 { // Skip some letters for performance
						edit2 := edit1[:i] + string(c) + edit1[i+1:]
						if dictionary.search(edit2) && !containsWord(candidates, edit2) {
							score := getWordScore(edit2, len(edit2))
							candidates = append(candidates, Candidate{edit2, 2, score})
							if len(candidates) >= MAX_CANDIDATES {
								return candidates
							}
						}
					}
				}
			}
		}
	}

	return candidates
}

// Generate all possible edits at distance 1
func generateAllEdits1(word string) []string {
	edits := []string{}

	// Deletions
	for i := 0; i < len(word); i++ {
		edits = append(edits, word[:i]+word[i+1:])
	}

	// Transpositions
	for i := 0; i < len(word)-1; i++ {
		edits = append(edits, word[:i]+string(word[i+1])+string(word[i])+word[i+2:])
	}

	// Limited substitutions (for performance)
	for i := 0; i < len(word); i++ {
		for c := 'a'; c <= 'z'; c += 3 { // Only use some letters
			edits = append(edits, word[:i]+string(c)+word[i+1:])
		}
	}

	return edits
}

// Calculate word score based on frequency and similarity
func getWordScore(word string, originalLen int) int {
	// Base score from frequency dictionary
	score := WordFrequency[word]
	if score == 0 {
		score = 1 // Minimum score
	}

	// Adjust score based on length similarity
	lenDiff := abs(len(word) - originalLen)
	if lenDiff == 0 {
		score += 100 // Bonus for same length
	} else {
		score -= lenDiff * 10 // Penalty for different length
	}

	// Prefer shorter words when score is similar
	score -= len(word)

	// Common words should get a boost
	commonWords := map[string]bool{
		"the": true, "is": true, "a": true, "an": true, "and": true,
		"are": true, "as": true, "at": true, "be": true, "but": true,
		"by": true, "for": true, "if": true, "in": true, "into": true,
		"it": true, "no": true, "not": true, "of": true, "on": true,
		"or": true, "such": true, "that": true, "their": true, "then": true,
		"there": true, "these": true, "they": true, "this": true, "to": true,
		"was": true, "will": true, "with": true, "he": true, "she": true,
		"some": true, "test": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "can": true, "could": true,
		"would": true, "should": true, "may": true, "might": true,
		"sentence": true, "typo": true, "check": true, "spell": true, "checker": true,
	}

	if commonWords[word] {
		score += 200 // Big boost for very common words
	}

	return score
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func containsWord(candidates []Candidate, word string) bool {
	for _, c := range candidates {
		if c.word == word {
			return true
		}
	}
	return false
}

func getClipboardText() string {
	// Retry several times in case clipboard is being used
	for i := 0; i < 3; i++ {
		r, _, _ := openClipboard.Call(0)
		if r == 0 {
			log.Println("Failed to open clipboard, retrying...")
			continue
		}

		defer closeClipboard.Call()
		h, _, _ := getClipboardData.Call(win.CF_UNICODETEXT)
		if h == 0 {
			log.Println("No text in clipboard")
			return ""
		}

		p := win.GlobalLock(win.HGLOBAL(h))
		if p == nil {
			log.Println("Failed to lock clipboard memory")
			return ""
		}

		defer win.GlobalUnlock(win.HGLOBAL(h))
		text := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(p))[:])
		return text
	}

	log.Println("Failed to access clipboard after multiple attempts")
	return ""
}

func setClipboardText(text string) {
	for i := 0; i < 3; i++ {
		r, _, _ := openClipboard.Call(0)
		if r == 0 {
			log.Println("Failed to open clipboard for writing, retrying...")
			continue
		}

		defer closeClipboard.Call()

		r, _, _ = emptyClipboard.Call()
		if r == 0 {
			log.Println("Failed to empty clipboard")
			return
		}

		utf16, err := syscall.UTF16FromString(text)
		if err != nil {
			log.Printf("Failed to convert text: %v", err)
			return
		}

		h := win.GlobalAlloc(win.GMEM_MOVEABLE, uintptr(len(utf16)*2))
		if h == 0 {
			log.Println("Failed to allocate memory")
			return
		}

		p := win.GlobalLock(h)
		if p == nil {
			log.Println("Failed to lock memory")
			win.GlobalFree(h)
			return
		}

		copy((*[1 << 20]uint16)(unsafe.Pointer(p))[:len(utf16)], utf16)
		win.GlobalUnlock(h)

		r, _, _ = setClipboardData.Call(win.CF_UNICODETEXT, uintptr(h))
		if r == 0 {
			log.Println("Failed to set clipboard data")
			win.GlobalFree(h)
			return
		}

		log.Println("Successfully updated clipboard with corrected text")
		return
	}

	log.Println("Failed to set clipboard after multiple attempts")
}

// Get a simple icon for the system tray
func getIcon() []byte {
	// Update the path to your converted icon file (e.g. favicon.ico or favicon.png)
	iconPath := "icon1.ico"
	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		log.Printf("Failed to load icon from %s: %v", iconPath, err)
		return nil
	}
	return iconData
}
