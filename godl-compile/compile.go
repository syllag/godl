package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"godl"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
	"text/tabwriter"

	_ "github.com/mattn/go-sqlite3"
)

// Version of the tool
var Version = "v.0.5-RC1"

var state struct {
	dirname              string
	dbname               string
	fullname             string
	db                   *sql.DB
	stats                bool
	relation             godl.Relation
	origins              []string
	inconsistencyDegrees []float64
	originIndexes        map[string]int
	objectPropertyNames  []string
}

func computeInconsistencyDegrees() {
	initInconsistencyDegrees()

	var tab [2]int

	for i := 0; i < len(state.origins); i++ {
		for j := i + 1; j < len(state.origins); j++ {
			tab[0] = i
			tab[1] = j
			computeInconsistencyDegree(tab[:])
		}
	}
}

func computeInconsistencyDegree(origins []int) {
	query := `SELECT value FROM '%s' WHERE (origin='` + state.origins[origins[0]] + `'`

	for i := 1; i < len(origins); i++ {
		table := state.origins[origins[i]]
		query = query + ` OR origin='` + table + `'`
	}

	query += ")"

	for _, table := range state.relation.Elements {
		subQuery := fmt.Sprintf(query, table)
		query1 := subQuery + " AND positive"
		query2 := subQuery + " AND NOT(positive)"
		queryUnion := query1 + "\nINTERSECT\n" + query2
		joinQuery := fmt.Sprintf(`SELECT MAX(weight), origin FROM (%s) AS joined, '%s' WHERE '%s'.value=joined.value GROUP BY origin`,
			queryUnion, table, table)

		rows, _ := state.db.Query(joinQuery)
		for rows.Next() {
			var w float64
			var o string

			if err := rows.Scan(&w, &o); err != nil {
				log.Fatal(err)
			}

			if index := state.originIndexes[o]; w > state.inconsistencyDegrees[index] {
				state.inconsistencyDegrees[index] = w
			}
		}
	}
}

func restoreConsistancy() {
	for i, val := range state.origins {
		if w := state.inconsistencyDegrees[i]; w > 0 {
			for _, table := range state.relation.Elements {
				cut(table, val, state.inconsistencyDegrees[i])
			}

			for _, table := range state.objectPropertyNames {
				cut(table, val, state.inconsistencyDegrees[i])
			}
		}
	}
}

func cut(table string, origin string, weight float64) {
	query := `DELETE FROM '%s' WHERE origin='%s' AND weight <= %f`
	query = fmt.Sprintf(query, table, origin, weight)

	state.db.Exec(query)
}

func parseFlags() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "godl-compile\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  godl-compile [arguments] db_name\n\n")
		fmt.Fprintf(os.Stderr, "arguments:\n")

		flag.PrintDefaults()
	}

	var help bool
	flag.BoolVar(&help, "h", false, "this message")

	var list bool
	flag.BoolVar(&list, "l", false, "list available databases")
	flag.BoolVar(&state.stats, "s", false, "print some stats")

	var version bool
	flag.BoolVar(&version, "v", false, "version")

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

func init() {
	usr, _ := user.Current()
	state.dirname = usr.HomeDir + "/" + "GoDL"

	parseFlags()
}

func openDB() {
	var err error
	state.db, err = sql.Open("sqlite3", state.fullname)
	log.Println("opening database", "'"+state.fullname+"'...")

	if err != nil {
		log.Fatal(err)
	}
}

func closeDB() {
	state.db.Close()
	log.Println("database closed.")
}

func importRelation() {
	query := `select value from __GoDL_JSON__ where name = 'TBox';`
	row := state.db.QueryRow(query)

	var raw string
	err := row.Scan(&raw)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	reader := strings.NewReader(raw)
	err = json.NewDecoder(reader).Decode(&state.relation)
}

func importObjectPropertyNames() {
	query := `select value from __GoDL_JSON__ where name = 'objectPropertyNames';`
	row := state.db.QueryRow(query)

	var raw string
	err := row.Scan(&raw)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	reader := strings.NewReader(raw)
	err = json.NewDecoder(reader).Decode(&state.objectPropertyNames)
}

func importOrigins() {
	query := `SELECT value FROM __GoDL_JSON__ WHERE name = 'origins';`
	row := state.db.QueryRow(query)

	var raw string
	err := row.Scan(&raw)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	reader := strings.NewReader(raw)
	err = json.NewDecoder(reader).Decode(&state.origins)

	state.originIndexes = make(map[string]int)

	for i, val := range state.origins {
		state.originIndexes[val] = i
	}
}

func populate() {
	for _, eqClass := range state.relation.EquivalentClasses {
		// 1 : on peuple les équivalents
		for i := 1; i < len(eqClass); i++ {
			tableSrc := state.relation.Elements[eqClass[i]]
			tableDst := state.relation.Elements[eqClass[0]]

			populateTable(tableSrc, tableDst, true)
		}

		for i := 1; i < len(eqClass); i++ {
			tableSrc := state.relation.Elements[eqClass[0]]
			tableDst := state.relation.Elements[eqClass[i]]

			populateTable(tableSrc, tableDst, true)
		}

		index := eqClass[0]
		tableSrc := state.relation.Elements[index]

		// 2 : on peuple les induits (positifs et négatifs)
		for i := 0; i < state.relation.Size; i++ {
			positive := state.relation.CompactIncidenceMatrix[index][i]
			switch positive {
			case 1:
				tableDst := state.relation.Elements[i]
				populateTable(tableSrc, tableDst, true)
			case -1:
				tableDst := state.relation.Elements[i]
				populateTable(tableSrc, tableDst, false)
			}
		}
	}

	// 3 : on met à jour les équivalents
	for _, eqClass := range state.relation.EquivalentClasses {
		for i := 1; i < len(eqClass); i++ {
			tableSrc := state.relation.Elements[eqClass[0]]
			tableDst := state.relation.Elements[eqClass[i]]

			copyIntoTable(tableSrc, tableDst)
		}
	}
}

func copyIntoTable(src string, dst string) {
	query := `INSERT OR IGNORE INTO '%s' SELECT value, positive, weight, origin FROM '%s';`
	query = fmt.Sprintf(query, dst, src)

	_, err := state.db.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func populateTable(src string, dst string, positive bool) {
	var query string

	if positive {
		query = `INSERT OR IGNORE INTO '%s' SELECT value, 1, weight, origin FROM '%s' WHERE positive = 1;`
	} else {
		query = `INSERT OR IGNORE INTO '%s' SELECT value, 0, weight, origin FROM '%s' WHERE positive = 1;`
	}

	query = fmt.Sprintf(query, dst, src)

	_, err := state.db.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func initInconsistencyDegrees() {
	state.inconsistencyDegrees = make([]float64, len(state.origins))
}

func printStats() {
	state.db.Exec("ANALYZE")

	query := "SELECT tbl, stat FROM sqlite_stat1 WHERE tbl NOT LIKE '%__GoDL%';"

	rows, _ := state.db.Query(query)

	log.Println("statistics:")

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)

	for rows.Next() {
		var table string
		var stats string

		if err := rows.Scan(&table, &stats); err != nil {
			log.Fatal(err)
		}

		fmt.Fprintf(w, "   %s\t%s\n", table, strings.SplitN(stats, " ", 2)[0])
	}

	fmt.Fprintln(w)

	w.Flush()
}

func main() {
	defer closeDB()
	fmt.Println("godl-compile")

	openDB()

	log.Println("reading database...")
	importOrigins()
	importRelation()
	importObjectPropertyNames()

	if state.stats {
		printStats()
	}

	log.Println("populating database...")
	populate()

	if state.stats {
		printStats()
	}

	log.Println("computing inconsistency degree(s)...")
	computeInconsistencyDegrees()
	if state.stats {
		log.Println("degrees:", state.inconsistencyDegrees)
	}

	log.Println("restoring consistency...")
	restoreConsistancy()

	if state.stats {
		printStats()
	}
	log.Print("computing degrees (for verification)... ")
	computeInconsistencyDegrees()
	log.Println("result:", state.inconsistencyDegrees)
}
