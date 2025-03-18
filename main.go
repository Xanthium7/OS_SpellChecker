package main

import (
	"bufio"
	"log"
	"os"
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
	MAX_CANDIDATES = 5
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

func loadDictionary(filePath string) {
	dictionary = newTrie()
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open dictionary file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		dictionary.insert(strings.ToLower(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to read dictionary file: %v", err)
	}
}

func main() {
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
			if !unicode.IsLetter(rune(cleanWord[i])) && !unicode.IsNumber(rune(cleanWord[i])) {
				suffix = string(cleanWord[i]) + suffix
				cleanWord = cleanWord[:i]
			} else {
				break
			}
		}

		// Skip correcting if the word is empty, a number, or very short
		if len(cleanWord) <= 1 || isNumber(cleanWord) {
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

	bestCandidate := word
	bestDistance := len(word) // Initialize with worst possible distance

	// Try edit distance 1 and 2
	for distance := 1; distance <= 2; distance++ {
		candidates := findCandidatesWithDistance(word, distance)
		if len(candidates) > 0 {
			// Find the candidate with the shortest word length
			for _, candidate := range candidates {
				// Prefer shorter words as they're often more common
				if candidate.distance < bestDistance ||
					(candidate.distance == bestDistance && len(candidate.word) < len(bestCandidate)) {
					bestDistance = candidate.distance
					bestCandidate = candidate.word
				}
			}
			break
		}
	}

	if bestCandidate != word {
		log.Printf("Corrected '%s' to '%s'", word, bestCandidate)
	} else {
		log.Printf("No match found for '%s'", word)
	}

	return bestCandidate
}

// Calculate Levenshtein distance between two strings
func levenshteinDistance(s, t string) int {
	m := len(s)
	n := len(t)
	d := make([][]int, m+1)
	for i := range d {
		d[i] = make([]int, n+1)
	}

	for i := 0; i <= m; i++ {
		d[i][0] = i
	}
	for j := 0; j <= n; j++ {
		d[0][j] = j
	}

	for j := 1; j <= n; j++ {
		for i := 1; i <= m; i++ {
			if s[i-1] == t[j-1] {
				d[i][j] = d[i-1][j-1]
			} else {
				min := d[i-1][j]
				if d[i][j-1] < min {
					min = d[i][j-1]
				}
				if d[i-1][j-1] < min {
					min = d[i-1][j-1]
				}
				d[i][j] = min + 1
			}
		}
	}

	return d[m][n]
}

func findCandidatesWithDistance(word string, maxDistance int) []Candidate {
	candidates := []Candidate{}

	// Try deletions
	for i := 0; i < len(word); i++ {
		newWord := word[:i] + word[i+1:]
		if dictionary.search(newWord) {
			candidates = append(candidates, Candidate{newWord, 1})
			if len(candidates) >= MAX_CANDIDATES {
				return candidates
			}
		}
	}

	// Try transpositions
	for i := 0; i < len(word)-1; i++ {
		newWord := word[:i] + string(word[i+1]) + string(word[i]) + word[i+2:]
		if dictionary.search(newWord) {
			candidates = append(candidates, Candidate{newWord, 1})
			if len(candidates) >= MAX_CANDIDATES {
				return candidates
			}
		}
	}

	// Try substitutions
	for i := 0; i < len(word); i++ {
		for c := 'a'; c <= 'z'; c++ {
			newWord := word[:i] + string(c) + word[i+1:]
			if dictionary.search(newWord) {
				candidates = append(candidates, Candidate{newWord, 1})
				if len(candidates) >= MAX_CANDIDATES {
					return candidates
				}
			}
		}
	}

	// Try insertions
	for i := 0; i <= len(word); i++ {
		for c := 'a'; c <= 'z'; c++ {
			newWord := word[:i] + string(c) + word[i:]
			if dictionary.search(newWord) {
				candidates = append(candidates, Candidate{newWord, 1})
				if len(candidates) >= MAX_CANDIDATES {
					return candidates
				}
			}
		}
	}

	// If we're allowed edit distance 2, try another level of edits
	// but only if we still need more candidates
	if maxDistance >= 2 && len(candidates) < MAX_CANDIDATES/2 {
		// Get all words with edit distance 1
		edits1 := []string{}

		// Add deletions
		for i := 0; i < len(word); i++ {
			edits1 = append(edits1, word[:i]+word[i+1:])
		}

		// Add transpositions
		for i := 0; i < len(word)-1; i++ {
			edits1 = append(edits1, word[:i]+string(word[i+1])+string(word[i])+word[i+2:])
		}

		// For each edit1 word, try another edit
		for _, edit1 := range edits1 {
			// Skip if we already found this word
			alreadyFound := false
			for _, c := range candidates {
				if c.word == edit1 {
					alreadyFound = true
					break
				}
			}
			if alreadyFound {
				continue
			}

			// Try another edit
			for i := 0; i < len(edit1); i++ {
				for c := 'a'; c <= 'z'; c++ {
					newWord := edit1[:i] + string(c) + edit1[i+1:]
					if dictionary.search(newWord) && !contains(candidates, newWord) {
						candidates = append(candidates, Candidate{newWord, 2})
						if len(candidates) >= MAX_CANDIDATES {
							return candidates
						}
					}
				}
			}
		}
	}

	return candidates
}

func contains(candidates []Candidate, word string) bool {
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
	// This is a simple 16x16 icon in ICO format
	// You can replace this with a proper icon file
	return []byte{
		0, 0, 1, 0, 1, 0, 16, 16, 0, 0, 1, 0, 24, 0, 104, 3,
		0, 0, 22, 0, 0, 0, 40, 0, 0, 0, 16, 0, 0, 0, 32, 0,
		0, 0, 1, 0, 24, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 255, 255, 255, 0, 0, 0, 255,
		0, 0, 0, 255, 0, 0, 0, 255, 0, 0, 0, 255, 0, 0, 0,
		255, 0, 0, 0, 255, 0, 0, 0, 255, 0, 0, 0, 255, 0, 0,
		0, 255, 0, 0, 0, 255, 0, 0, 0, 255, 0, 0, 0, 255, 0,
		0, 0, 255, 0, 0, 0, 255, 0, 0, 0, 255, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 128, 255,
		64, 128, 255, 64, 128, 255, 64, 128, 255, 64, 128, 255,
		64, 128, 255, 64, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 64, 128, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		64, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 128, 255, 255, 255,
		255, 64, 128, 255, 64, 128, 255, 64, 128, 255, 255, 255,
		255, 64, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 128, 255, 255, 255,
		255, 64, 128, 255, 0, 0, 0, 64, 128, 255, 255, 255, 255,
		64, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 128, 255, 255, 255,
		255, 64, 128, 255, 64, 128, 255, 64, 128, 255, 255, 255,
		255, 64, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 128, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 64, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 64, 128, 255,
		64, 128, 255, 64, 128, 255, 64, 128, 255, 64, 128, 255,
		64, 128, 255, 64, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 128, 255, 0, 128, 255, 0,
		128, 255, 0, 128, 255, 0, 128, 255, 0, 128, 255, 0, 128,
		255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 128, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		0, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 128, 255, 255, 255, 255,
		0, 128, 255, 0, 128, 255, 0, 128, 255, 255, 255, 255, 0,
		128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 128, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		0, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 128, 255, 255, 255, 255, 0,
		128, 255, 0, 128, 255, 0, 128, 255, 255, 255, 255, 0,
		128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 128, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		0, 128, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 128, 255, 0, 128, 255, 0,
		128, 255, 0, 128, 255, 0, 128, 255, 0, 128, 255, 0, 128,
		255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		255, 255, 0, 255, 255, 0, 255, 255, 0, 255, 255, 0,
		255, 255, 0, 255, 255, 0, 255, 255, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 255, 0, 255, 255, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 255, 255, 255, 255, 255, 0, 255, 255, 0, 0, 0, 255,
		255, 255, 255, 255, 255, 0, 255, 255, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 0, 255, 255, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255,
		255, 255, 255, 255, 0, 255, 255, 0, 0, 0, 255, 255,
		255, 255, 255, 255, 0, 255, 255, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255,
		255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
		255, 255, 255, 255, 255, 0, 255, 255, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255,
		255, 0, 255, 255, 0, 255, 255, 0, 255, 255, 0, 255,
		255, 0, 255, 255, 0, 255, 255, 0, 0, 0, 0,
	}
}
