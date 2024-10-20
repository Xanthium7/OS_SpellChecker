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

	// Check for edit distances up to 3
	for distance := 1; distance <= 3; distance++ {
		candidates = append(candidates, findCandidates(word, distance)...)
		if len(candidates) > 0 {
			break
		}
	}

	log.Printf("Candidates found: %v", candidates)

	if len(candidates) > 0 {
		return candidates[0] // Return the first candidate
	}

	log.Printf("No match found for '%s'", word)
	return word // If no match found, return the original word
}

func findCandidates(word string, maxDistance int) []string {
	candidates := []string{}
	queue := []struct {
		word     string
		distance int
	}{{word, 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.distance > maxDistance {
			continue
		}

		if dictionary.search(current.word) {
			candidates = append(candidates, current.word)
			continue
		}

		if current.distance == maxDistance {
			continue
		}

		// Generate all possible edits
		for i := 0; i <= len(current.word); i++ {
			// Deletions
			if i < len(current.word) {
				newWord := current.word[:i] + current.word[i+1:]
				queue = append(queue, struct {
					word     string
					distance int
				}{newWord, current.distance + 1})
			}

			// Insertions
			for ch := 'a'; ch <= 'z'; ch++ {
				newWord := current.word[:i] + string(ch) + current.word[i:]
				queue = append(queue, struct {
					word     string
					distance int
				}{newWord, current.distance + 1})
			}

			// Substitutions
			if i < len(current.word) {
				for ch := 'a'; ch <= 'z'; ch++ {
					newWord := current.word[:i] + string(ch) + current.word[i+1:]
					queue = append(queue, struct {
						word     string
						distance int
					}{newWord, current.distance + 1})
				}
			}

			// Transpositions
			if i < len(current.word)-1 {
				newWord := current.word[:i] + string(current.word[i+1]) + string(current.word[i]) + current.word[i+2:]
				queue = append(queue, struct {
					word     string
					distance int
				}{newWord, current.distance + 1})
			}
		}
	}

	return candidates
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
