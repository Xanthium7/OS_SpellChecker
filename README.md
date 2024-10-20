# Spell Checker

This project is a simple spell checker that reads the clipboard content, checks for spelling mistakes, and updates the clipboard with corrected text.

## Features

- Reads clipboard content
- Checks for spelling mistakes using a dictionary
- Updates clipboard with corrected text


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

## Inserting Words
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

## Searching for Words
When checking if a word exists, the program traverses the Trie character by character.<br ></br> If any character of the word is missing in the Trie, the search fails,<br ></br> and the word is considered misspelled . If it reaches the last character and the isEnd flag is true, the word is valid.

## Upgrade

- more the words in the dicitonary the better
