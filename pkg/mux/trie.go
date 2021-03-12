/*
MIT License

Copyright (c) 2019 bmf

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package mux

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Tree is a trie tree.
type Tree struct {
	namespace map[string]*Node
}

// Node is a node of tree.
type Node struct {
	label    string
	handlers []interface{}
	children map[string]*Node
}

// Param is parameter.
type Param struct {
	key   string
	value string
}

// Params is parameters.
type Params []*Param

// Result is a search result.
type Result struct {
	Handlers []interface{}
	Params   Params
}

const (
	pathDelimiter     = "/"
	paramDelimiter    = ":"
	leftPtnDelimiter  = "["
	rightPtnDelimiter = "]"
	ptnWildcard       = "(.+)"
)

// NewTree creates a new trie tree.
func NewTree(namespaces ...string) *Tree {
	namespace := make(map[string]*Node, len(namespaces))
	for _, name := range namespaces {
		namespace[name] = &Node{
			label:    "",
			handlers: nil,
			children: make(map[string]*Node),
		}
	}
	return &Tree{namespace}
}

// Insert inserts a route definition to tree.
func (t *Tree) Insert(namespace string, path string, handler interface{}) error {
	curNode, present := t.namespace[namespace]

	if !present {
		curNode = &Node{
			label:    "",
			handlers: nil,
			children: make(map[string]*Node),
		}
		t.namespace[namespace] = curNode
	}

	if path == pathDelimiter {
		if len(curNode.label) != 0 && curNode.handlers == nil {
			return errors.New("Root node already exists")
		}

		curNode.label = path
		if curNode.handlers == nil {
			curNode.handlers = []interface{}{handler}
		} else {
			curNode.handlers = append(curNode.handlers, handler)
		}

		return nil
	}

	for _, l := range deleteEmpty(strings.Split(path, pathDelimiter)) {
		if nextNode, ok := curNode.children[l]; ok {
			curNode = nextNode
		} else {
			curNode.children[l] = &Node{
				label:    l,
				handlers: []interface{}{handler},
				children: make(map[string]*Node),
			}

			curNode = curNode.children[l]
		}
	}

	return nil
}

type regCache struct {
	s sync.Map
}

// Get gets a compiled regexp from cache or create it.
func (rc *regCache) Get(ptn string) (*regexp.Regexp, error) {
	v, ok := rc.s.Load(ptn)
	if ok {
		reg, ok := v.(*regexp.Regexp)
		if !ok {
			return nil, fmt.Errorf("the value of %q is wrong", ptn)
		}
		return reg, nil
	}
	reg, err := regexp.Compile(ptn)
	if err != nil {
		return nil, err
	}
	rc.s.Store(ptn, reg)
	return reg, nil
}

var regC = &regCache{}

// Search searches a path from a tree.
func (t *Tree) Search(namespace string, path string) (*Result, error) {
	var params Params

	n := t.namespace[namespace]

	if len(n.label) == 0 && len(n.children) == 0 {
		return nil, errors.New("tree is empty")
	}

	label := deleteEmpty(strings.Split(path, pathDelimiter))
	curNode := n

	for _, l := range label {
		if nextNode, ok := curNode.children[l]; ok {
			curNode = nextNode
		} else {
			// pattern matching priority depends on an order of routing definition
			// ex.
			// 1 /foo/:id
			// 2 /foo/:id[^\d+$]
			// 3 /foo/:id[^\w+$]
			// priority is 1, 2, 3
			if len(curNode.children) == 0 {
				return &Result{}, errors.New("handler is not registered")
			}

			count := 0
			for c := range curNode.children {
				if string([]rune(c)[0]) == paramDelimiter {
					ptn := getPattern(c)

					reg, err := regC.Get(ptn)
					if err != nil {
						return nil, err
					}
					if reg.Match([]byte(l)) {
						param := getParameter(c)
						params = append(params, &Param{
							key:   param,
							value: l,
						})

						curNode = curNode.children[c]
						count++
						break
					} else {
						return &Result{}, errors.New("param does not match")
					}
				}

				count++

				// If no match is found until the last loop.
				if count == len(curNode.children) {
					return &Result{}, errors.New("handler is not registered")
				}
			}
		}
	}

	if curNode.handlers == nil {
		return &Result{}, errors.New("handler is not registered")
	}

	return &Result{
		Handlers: curNode.handlers,
		Params:   params,
	}, nil
}

// getPattern gets a pattern from a label.
// ex.
// :id[^\d+$] → ^\d+$
// :id        → (.+)
func getPattern(label string) string {
	leftI := strings.Index(label, leftPtnDelimiter)
	rightI := strings.Index(label, rightPtnDelimiter)

	// if label has not pattern, return wild card pattern as default.
	if leftI == -1 || rightI == -1 {
		return ptnWildcard
	}

	return label[leftI+1 : rightI]
}

// getParameter gets a parameter from a label.
// ex.
// :id[^\d+$] → id
// :id        → id
func getParameter(label string) string {
	leftI := strings.Index(label, paramDelimiter)
	rightI := func(l string) int {
		r := []rune(l)

		var n int

		for i := 0; i < len(r); i++ {
			n = i
			if string(r[i]) == leftPtnDelimiter {
				n = i
				break
			} else if i == len(r)-1 {
				n = i + 1
				break
			}
		}

		return n
	}(label)

	return label[leftI+1 : rightI]
}

func deleteEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}
