package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"regexp"
	"strings"
	"text/tabwriter"

	_ "github.com/mattn/go-sqlite3"
)

// Version of the tool
var Version = "v.0.5-RC1"

var state struct {
	dirname  string
	dbname   string
	fullname string
	db       *sql.DB
	reQ      *regexp.Regexp
	reArgs   *regexp.Regexp
}

type predicate struct {
	positive     bool
	name         string
	args         []string
	arity        int
	nbVariables  int
	nbUnderscore int
}

func getVariables(p predicate) map[string]int {
	m := make(map[string]int)

	for i, val := range p.args {
		m[val] = i
	}

	return m
}

func parsePredicates(q string) []predicate {
	var res []predicate // := make([]predicate, 0)

	// reH := state.reH
	// head := reH.FindStringSubmatch(q)[1]

	reQ := state.reQ
	queue := reQ.FindAllStringSubmatch(q, -1)

	for _, val := range queue {
		res = append(res, parsePredicate(val[1:]))
	}

	return res
}

func parsePredicate(rawS []string) predicate {
	var p predicate

	p.positive = rawS[0] != "!"
	p.name = rawS[1]
	p.args = strings.Split(rawS[2], ",")
	p.arity = len(p.args)
	p.nbVariables = 0
	p.nbUnderscore = 0

	for i := range p.args {
		p.args[i] = strings.Trim(p.args[i], "\t ")
		if p.args[i][0] == '?' {
			p.nbVariables++

			if p.name != "q" && (p.nbVariables > 1 && p.nbUnderscore != 1) {
				fmt.Println(p)
				log.Fatal("Fatal error (not in fragment): '", p.name, "'")
			}

		}
		if p.args[i] == "_" {
			p.nbUnderscore++
		}
	}

	return p
}

// place 0 : unary predicate
// place 1 : binary predicate left
// place 2 : binary predicate right

func firstOccurenceOf(variable string, predicates []predicate) (pRes predicate, index int) { //(int, string) {
	for i, p := range predicates {
		for _, arg := range p.args {
			if arg == variable {
				return p, i
				// switch  {
				// case p.arity == 1:
				// 	return i, p.name + "__GODL__" + strconv.Itoa(j)
				// case p.arity == 2 && p.nbUnderscore == 0:
				// 	return i + 1, p.name + "__GODL__" + strconv.Itoa(j)
				// case p.arity == 2 && p.nbUnderscore == 1 && p.args[0] == "_":
				// 	return i + 1, p.name + "__GODL_RIGHT____GODL__" + strconv.Itoa(j)
				// case p.arity == 2 && p.nbUnderscore == 1 && p.args[1] == "_":
				// 	return i + 1, p.name + "__GODL_LEFT____GODL__" + strconv.Itoa(j)
				// }
			}
		}
	}

	return pRes, -1 //-1, ""
}

func tableName(p predicate) string {
	var name string

	if p.arity == 2 && p.nbUnderscore == 1 {
		if p.args[0] == "_" {
			name = p.name + "__GoDL_RIGHT__"
		} else {
			name = p.name + "__GoDL_LEFT__"
		}
	} else {
		name = p.name
	}

	return name
}

func wherePart(p predicate, i int) string {
	var wp string

	tbn := fmt.Sprintf("%s__GODL__%d", tableName(p), i)

	switch {
	case p.arity == 1:
		varName := p.args[0]
		varName = varName[1:len(varName)]
		wp += varName + "='" + tbn + "'.value AND "
	case p.arity == 2 && p.nbUnderscore == 0 && p.nbVariables == 2:
		varName1 := p.args[0]
		varName1 = varName1[1:len(varName1)]

		varName2 := p.args[1]
		varName2 = varName2[1:len(varName2)]

		wp += varName1 + "='" + tbn + "'.leftValue AND "
		wp += varName2 + "='" + tbn + "'.rightValue AND "

	case p.arity == 2 && p.nbUnderscore == 0 && p.nbVariables == 1:
		name1 := p.args[0]
		name2 := p.args[1]
		if name1[0] == '?' {
			name1 = name1[1:len(name1)]
			name2 = "'" + name2 + "'"
		} else {
			name1 = "'" + name1 + "'"
			name2 = name2[1:len(name2)]
		}
		wp += name1 + "='" + tbn + "'.leftValue AND "
		wp += name2 + "='" + tbn + "'.rightValue AND "

	case p.arity == 2 && p.nbUnderscore == 1 && p.nbVariables == 1:
		if p.args[0] != "_" {
			varName := p.args[0]
			varName = varName[1:len(varName)]
			wp += varName + "='" + tbn + "'.Value AND "
		} else {
			varName := p.args[1]
			varName = varName[1:len(varName)]
			wp += varName + "='" + tbn + "'.Value AND "
		}

	default:
		log.Fatal("don't understand for predicate " + p.name)
	}
	if p.positive {
		wp += "'" + tbn + "'.positive"
	} else {
		wp += "NOT('" + tbn + "'.positive)"
	}

	return wp
}

func buildQuery(head predicate, queue []predicate, variables map[string]int) string {
	//fmt.Println(head, queue)

	query := `SELECT`
	for i, varName := range head.args {
		p, index := firstOccurenceOf(varName, queue)
		if index == -1 {
			log.Fatal("Fatal Error: variable '" + varName + "' not found in queue...")
		}
		name := tableName(p) + "__GODL__"
		varName = varName[1:len(varName)]

		switch {
		case p.arity == 1 || (p.arity == 2 && p.nbUnderscore == 1):
			query = fmt.Sprintf("%s '%s%d'.value AS %s", query, name, index, varName)
		case p.arity == 2 && p.args[0][0] == '?' && p.nbVariables == 1:
			query = fmt.Sprintf("%s '%s%d'.leftValue AS %s", query, name, index, varName)
		case p.arity == 2 && p.args[1][0] == '?' && p.nbVariables == 1:
			query = fmt.Sprintf("%s '%s%d'.rightValue AS %s", query, name, index, varName)
		case p.nbVariables == 2 && varName == p.args[0][1:]:
			query = fmt.Sprintf("%s '%s%d'.leftValue AS %s", query, name, index, varName)
		case p.nbVariables == 2 && varName == p.args[1][1:]:
			query = fmt.Sprintf("%s '%s%d'.rightValue AS %s", query, name, index, varName)
		}

		if i != len(head.args)-1 {
			query += ","
		}
	}

	query += "\nFROM"

	//fmt.Println(queue)

	for i, p := range queue {
		tbn := tableName(p)

		query = fmt.Sprintf("%s '%s' AS '%s__GODL__%d',", query, tbn, tbn, i)
	}
	query = query[:len(query)-1] + "\n"

	// fmt.Println(queue)

	query += "WHERE\n"
	for i, p := range queue {
		query += "  " + wherePart(p, i)
		if i != len(queue)-1 {
			query += " AND "
		}
		query += "\n"
	}

	// query += ";"

	return query
}

func execQuery(query string, n int) {
	vals := make([]string, n)
	s := make([]interface{}, n)

	for i := range vals {
		s[i] = &(vals[i])
	}

	rows, err := state.db.Query(query)

	if err != nil {
		log.Println(query)
		log.Fatal(err)
	}

	log.Println("results:")

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 1, 3, 0, '\t', 0)

	for rows.Next() {
		rows.Scan(s...)

		if err != nil {
			log.Println(query)
			log.Fatal(err)
		}

		for i := 0; i < n; i++ {
			fmt.Fprint(w, vals[i])

			if i != n-1 {
				fmt.Fprint(w, "\t")
			}
		}

		fmt.Fprintln(w)
	}

	w.Flush()

	fmt.Println()
}

func treatQuery(q string) {
	log.Println("treating:", q)

	// TODO: verify structure
	// TODO: verify query
	// TODO: verify head

	q = strings.TrimSpace(q)

	predicates := parsePredicates(q)

	head := predicates[0]
	queue := predicates[1:]
	variables := getVariables(head)

	query := buildQuery(head, queue, variables)

	execQuery(query, len(variables))
}

func initRegexps() {
	// state.reQ = regexp.MustCompile(`\s*q\s*(\(.*?\))\s*:-.*`)
	// state.reH = regexp.MustCompile(`(q\(.*?\))\s*:-.*`)
	state.reQ = regexp.MustCompile(`(!?)\s*([\w:]+)\s*\((.+?)\)`)
}

func parseFlags() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "godl-query\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  godl-query db_name\n\n")
		fmt.Fprintf(os.Stderr, "arguments:\n")

		flag.PrintDefaults()
	}

	var help bool
	flag.BoolVar(&help, "h", false, "this message")

	var version bool
	flag.BoolVar(&version, "v", false, "version")

	var list bool
	flag.BoolVar(&list, "l", false, "list available databases")

	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(0)
	}

	if version {
		fmt.Println("godl-query version:", Version)
		os.Exit(0)
	}

	if list {
		fmt.Println("\033[1mAvailable databases:\033[0m")
		files, _ := ioutil.ReadDir(state.dirname)
		for i, f := range files {
			fmt.Printf("(%d) %s\t%d\n", i, f.Name(), f.Size())
		}
		os.Exit(0)
	}

	if flag.NArg() > 0 {
		state.dbname = flag.Arg(0)
	} else {
		state.dbname = "noname.sqlite3"
	}
	state.fullname = state.dirname + string(os.PathSeparator) + state.dbname

}

func openDB() {
	var err error
	state.db, err = sql.Open("sqlite3", state.fullname)
	log.Println("opening database", "'"+state.fullname+"'...")

	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	usr, _ := user.Current()
	state.dirname = usr.HomeDir + "/" + "GoDL"

	parseFlags()

	openDB()
	initRegexps()
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	//fmt.Print("Enter query (C-D to quit): ")

	query, err := reader.ReadString('\n')
	for err != io.EOF {

		query = strings.TrimSpace(query)

		if len(query) != 0 && query[0] != '#' {
			treatQuery(query)
		}

		query, err = reader.ReadString('\n')
	}
}
