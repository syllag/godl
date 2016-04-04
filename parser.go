// godl is a description logic toolkit in go.

package godl

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// DLPredicate : Basic structure to stock predicates
type DLPredicate struct {
	Name      string
	Arguments []DLPredicate
}

func isPredicate(s string) bool {
	return len(s) > 1 && s[len(s)-1] == '('
}

func preProc(s string) string {
	re := regexp.MustCompile(`\s*\(`)
	s = re.ReplaceAllString(s, "( ")
	s = strings.Replace(s, ")", " ) ", -1)

	return s
}

func parse(scanner *bufio.Scanner, name string) (result DLPredicate) {
	var pred DLPredicate

	result.Name = name
	result.Arguments = make([]DLPredicate, 0)

	ok := scanner.Scan()
	token := scanner.Text()

	for token != ")" && ok {
		if isPredicate(token) {
			pred = parse(scanner, strings.TrimRight(token, "("))
		} else if token[0] == '"' {
			pred.Name = token
			for token[len(token)-1] != '"' && ok {
				pred.Name += " "
				ok = scanner.Scan()
				token = scanner.Text()
				pred.Name += token
			}
		} else {
			pred.Name = token
			pred.Arguments = make([]DLPredicate, 0)
		}

		result.Arguments = append(result.Arguments, pred)

		ok = scanner.Scan()
		token = scanner.Text()
	}

	return result
}

// FindOntology : finds the ontology predicate
func (p *DLPredicate) FindOntology() *DLPredicate {
	for i := range p.Arguments {
		if p.Arguments[i].Name == "Ontology" {
			return &p.Arguments[i]
		}

	}

	return nil
}

// InfixString : Prinst in infix form
// TODO strings.Join
func (p *DLPredicate) InfixString(depth int) string {
	var buffer bytes.Buffer

	buffer.WriteString(p.Name)

	if len(p.Arguments) > 0 {
		if depth > 0 {
			buffer.WriteString("(")
		}
	}

	for i, p2 := range p.Arguments {
		if p.Name == "Ontology" {
			buffer.WriteString("\n   ")
		}
		if i != 0 && depth > 1 {
			buffer.WriteString(" ")
		}
		buffer.WriteString(p2.InfixString(depth + 1))
	}

	if p.Name == "Ontology" {
		buffer.WriteString("\n")
	}

	if len(p.Arguments) > 0 && depth > 0 {
		buffer.WriteString(")")
	}

	if depth == 1 {
		buffer.WriteString("\n")
	}

	return buffer.String()
}

// Parse : parse a string formatted in ansiColorGreen
// BUG potential preproccessing problem for strings "example (2 blanks  )"
// TODO correct this problem ;-)
func Parse(s string) DLPredicate {
	s = preProc(s)

	// création du scanner
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Split(bufio.ScanWords)

	result := parse(scanner, "")

	return result
}
