package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"syscall"
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
)

const (
	MOD_ALT  = 0x0001
	MOD_CTRL = 0x0002
	VK_S     = 0x53 // Virtual key code for 'S'
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
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("Spell Checker")
	systray.SetTooltip("Copy text, then click here to check spelling")
	mSpellCheck := systray.AddMenuItem("Check Clipboard Spelling", "Check spelling of clipboard text")
	go func() {
		for {
			select {
			case <-mSpellCheck.ClickedCh:
				checkSpelling()
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

func correctSpelling(text string) string {
	words := strings.Fields(text)
	var correctedWords []string
	for _, word := range words {
		correctedWord := findClosestMatch(strings.ToLower(word))
		if correctedWord != "" {
			correctedWords = append(correctedWords, correctedWord)
		} else {
			correctedWords = append(correctedWords, word)
		}
	}
	return strings.Join(correctedWords, " ")
}

func findClosestMatch(word string) string {
	log.Printf("Finding closest match for: %s", word)

	// Remove trailing punctuation
	word = strings.TrimRight(word, ".!?,:;")

	if dictionary.search(word) {
		log.Printf("Word '%s' found in dictionary", word)
		return word
	}

	candidates := []string{}

	// Check for edit distance 1 and 2
	for distance := 1; distance <= 2; distance++ {
		// Deletions
		for i := 0; i < len(word); i++ {
			candidate := word[:i] + word[i+1:]
			if dictionary.search(candidate) {
				candidates = append(candidates, candidate)
			}
		}

		// Substitutions
		for i := 0; i < len(word); i++ {
			for ch := 'a'; ch <= 'z'; ch++ {
				candidate := word[:i] + string(ch) + word[i+1:]
				if dictionary.search(candidate) {
					candidates = append(candidates, candidate)
				}
			}
		}

		// Insertions
		for i := 0; i <= len(word); i++ {
			for ch := 'a'; ch <= 'z'; ch++ {
				candidate := word[:i] + string(ch) + word[i:]
				if dictionary.search(candidate) {
					candidates = append(candidates, candidate)
				}
			}
		}

		// Transpositions (for edit distance 2)
		if distance == 2 {
			for i := 0; i < len(word)-1; i++ {
				candidate := word[:i] + string(word[i+1]) + string(word[i]) + word[i+2:]
				if dictionary.search(candidate) {
					candidates = append(candidates, candidate)
				}
			}
		}

		if len(candidates) > 0 {
			break // Stop if we found candidates
		}
	}

	log.Printf("Candidates found: %v", candidates)

	if len(candidates) > 0 {
		return candidates[0] // Return the first candidate
	}

	log.Printf("No match found for '%s'", word)
	return word // If no match found, return the original word
}
func getClipboardText() string {
	openClipboard.Call(0)
	defer closeClipboard.Call()
	h, _, _ := getClipboardData.Call(win.CF_UNICODETEXT)
	if h == 0 {
		return ""
	}
	p := win.GlobalLock(win.HGLOBAL(h))
	defer win.GlobalUnlock(win.HGLOBAL(h))
	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(p))[:])
}

func setClipboardText(text string) {
	openClipboard.Call(0)
	defer closeClipboard.Call()
	emptyClipboard.Call()
	utf16, _ := syscall.UTF16FromString(text)
	h := win.GlobalAlloc(win.GMEM_MOVEABLE, uintptr(len(utf16)*2))
	p := win.GlobalLock(h)
	copy((*[1 << 20]uint16)(unsafe.Pointer(p))[:], utf16)
	win.GlobalUnlock(h)
	setClipboardData.Call(win.CF_UNICODETEXT, uintptr(h))
}
