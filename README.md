# Spell Checker

This project is a simple spell checker that reads the clipboard content, checks for spelling mistakes, and updates the clipboard with corrected text.

## Features

- Reads clipboard content
- Checks for spelling mistakes using a dictionary
- Updates clipboard with corrected text if the word available in dicitonary


## TRIE YEAH!

A Trie is a tree-like data structure used to store a dynamic set or associative array where the keys are usually strings. It provides efficient search operations, making it ideal for tasks like spell checking.

### **TrieNode Structure**
Each node (`TrieNode`) holds:
- A map (`children`) of its child nodes, where the key is a character and the value is the next TrieNode.
- A boolean flag (`isEnd`) that indicates whether the current node marks the end of a valid word in the dictionary.

```go

type TrieNode struct {
    children map[rune]*TrieNode
    isEnd    bool
}

```

### Inserting Words
When you load words from the dictionary file, each word is inserted into the Trie character by character. <br ></br> If a character doesn't already have a corresponding child node,  <br ></br>a new one is created. The isEnd flag is set to true when the final character of the word is inserted, indicating the end of a valid word.

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

### Searching for Words
When checking if a word exists, the program traverses the Trie character by character.<br ></br> If any character of the word is missing in the Trie, the search fails,<br ></br> and the word is considered misspelled . If it reaches the last character and the isEnd flag is true, the word is valid.


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

## How its used?

### Dictionary Loading
When the program starts, it loads words from the dictionary.txt file into the Trie using the insert() function.<br ></br> Each word is broken down into characters and inserted into the Trie.


```go
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
```

### Spell Checking
When the user clicks "Check Clipboard Spelling" the program retrieves the clipboard text and splits it into words.<br ></br> For each word, the program searches the Trie to see if it's present in the dictionary. <br ></br> If the word is not found, it suggests the closest match (by checking edit distance and possible corrections).

```go
func checkSpelling() {
    text := getClipboardText()
    if text == "" {
        return
    }
    correctedText := correctSpelling(text)
    setClipboardText(correctedText)
}

```

#### The Trie allows for fast word lookups and efficient storage of a large number of words, making it ideal for spell-checking applications.<br ></br> It ensures that searching for a word takes O(length of word) time, making it much faster than scanning through a list of words.

## Upgrade

- Expand the Dictionary:<br></br>Add  QUALITY COMMON words to your dictionary.txt file to improve the accuracy of the spell checker.<br></br>

