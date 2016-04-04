package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"godl"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"os/user"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

// Version of the tool
var Version = "v.0.5-RC1"

const ansiColorGreen string = "\x1b[32m"
const ansiColorReset string = "\x1b[0m"

var _properties struct {
	dirname             string
	dbname              string
	fullname            string
	tbox                string
	aboxes              []string
	db                  *sql.DB
	doNotImportTBox     bool
	weightGenerator     func(int) float64
	classNames          []string
	objectPropertyNames []string
	Debug               bool
}

type _TBoxDescriptor struct {
	classes             []string
	classesMap          map[string]int
	objectProperties    []string
	objectPropertiesMap map[string]int
	dataProperties      []string
	pass                uint
	todo                map[string]int
	relation            *godl.Relation
}

var tbox _TBoxDescriptor

func createDirectory() {
	dirname := _properties.dirname
	log.Println("creating directory", "'"+dirname+"'...")
	os.MkdirAll(dirname, 0755)
}

func createDB() {
	var err error

	//log.Println("Creating Database", "'"+_properties.dbname+"'...")
	log.Println("creating database", "'"+_properties.fullname+"'...")

	_properties.db, err = sql.Open("sqlite3", _properties.fullname)

	if err != nil {
		log.Fatal(err)
	}

	_properties.db.Exec("CREATE TABLE '__GoDL_JSON__' (name TEXT, value TEXT);")
}

func destroyDB() bool {
	return os.Remove(_properties.fullname) == nil
}

func closeDB() {
	_properties.db.Close()
	log.Println("database closed.")
}

func importABoxes() bool {
	log.Println("importing ABoxes...")

	for _, fn := range _properties.aboxes {
		bs, err := ioutil.ReadFile(fn)

		if err != nil {
			l := log.New(os.Stderr, "", 0)
			l.Println(err)
			os.Exit(1)
		}

		log.Println("importing ABox", fn)
		result := godl.Parse(string(bs))
		importABox(&result, fn)
	}

	return true
}

func importABox(predicates *godl.DLPredicate, filename string) bool {
	db := _properties.db
	ontology := predicates.FindOntology()
	passed := make(map[string]int)

	tx, _ := db.Begin()
	n := 1

	for i := range ontology.Arguments {
		weight := _properties.weightGenerator(n)

		switch ontology.Arguments[i].Name {
		case "ClassAssertion":
			className := ontology.Arguments[i].Arguments[0].Name
			value := ontology.Arguments[i].Arguments[1].Name
			request := fmt.Sprintf("INSERT OR IGNORE INTO '%s' VALUES ('%s', 1, %f, '%s')", className, value, weight, filename)
			db.Exec(request)

			n++

		case "ObjectPropertyAssertion":
			className := ontology.Arguments[i].Arguments[0].Name
			leftValue := ontology.Arguments[i].Arguments[1].Name
			rightValue := ontology.Arguments[i].Arguments[2].Name

			request := fmt.Sprintf("INSERT OR IGNORE INTO '%s' VALUES ('%s', '%s', 1, %f, '%s');",
				className, leftValue, rightValue, weight, filename)
			db.Exec(request)
			n++

			request = fmt.Sprintf("INSERT OR IGNORE INTO '%s__GoDL_LEFT__' VALUES ('%s', 1, %f, '%s');",
				className, leftValue, weight, filename)
			db.Exec(request)
			n++

			request = fmt.Sprintf("INSERT OR IGNORE INTO '%s__GoDL_RIGHT__' VALUES ('%s', 1, %f, '%s');",
				className, rightValue, weight, filename)
			db.Exec(request)
			n++

		default:
			predicate := ontology.Arguments[i].Name
			if _, ok := passed[predicate]; ok {
				passed[predicate]++
			} else {
				passed[predicate] = 1
			}
		}
	}
	state := tx.Commit()

	if state != nil {
		log.Panic(state)
	}

	for p, occ := range passed {
		log.Println("Warning: treatment of", "'"+p+"'", "not implemented ("+strconv.Itoa(occ), "occurences)")
	}

	return state != nil
}

func importTBox() bool {
	log.Println("importing TBox", _properties.tbox)

	bs, err := ioutil.ReadFile(_properties.tbox)

	if err != nil {
		l := log.New(os.Stderr, "", 0)
		l.Println(err)
		os.Exit(1)
	}
	result := godl.Parse(string(bs))
	return ImportTBox(&result)
}

// ImportTBox imports the TBOxes described in predicates
func ImportTBox(predicates *godl.DLPredicate) bool {
	tbox.classes = make([]string, 0)
	tbox.objectProperties = make([]string, 0)
	tbox.todo = make(map[string]int)

	ontology := predicates.FindOntology()

	for i := range ontology.Arguments {
		if ontology.Arguments[i].Name == "Declaration" {
			declaration := &ontology.Arguments[i].Arguments[0]

			switch declaration.Name {
			case "Class":
				tbox.classes = append(tbox.classes, declaration.Arguments[0].Name)
			case "ObjectProperty":
				tbox.objectProperties = append(tbox.objectProperties, declaration.Arguments[0].Name)
				tbox.classes = append(tbox.classes, declaration.Arguments[0].Name+"__GoDL_LEFT__")
				tbox.classes = append(tbox.classes, declaration.Arguments[0].Name+"__GoDL_RIGHT__")
			case "DataProperty":
				tbox.dataProperties = append(tbox.dataProperties, declaration.Arguments[0].Name)

			default:
				tbox.pass++
			}
		}
	}

	tbox.relation = godl.NewRelation(len(tbox.classes))
	if _properties.Debug {
		tbox.relation.Debug = true
	}

	for _, e := range tbox.classes {
		tbox.relation.AddElement(e)
	}

	for i := range ontology.Arguments {
		predicate := &ontology.Arguments[i]

		switch predicate.Name {
		case "SubClassOf":
			left := predicate.Arguments[0].Name
			right := predicate.Arguments[1].Name
			tbox.relation.SetSubClassOf(left, right)
		case "DisjointClasses":
			left := predicate.Arguments[0].Name
			right := predicate.Arguments[1].Name
			tbox.relation.SetDisjointClasses(left, right)
		case "EquivalentClasses":
			left := predicate.Arguments[0].Name
			right := predicate.Arguments[1].Name
			tbox.relation.SetSubClassOf(left, right)
			tbox.relation.SetSubClassOf(right, left)
		case "ObjectComplementOf":
			log.Panic("Not implemented")
		case "ObjectPropertyDomain":
			left := predicate.Arguments[0].Name + "__GoDL_LEFT__"
			right := predicate.Arguments[1].Name
			tbox.relation.SetSubClassOf(left, right)
		case "ObjectPropertyRange":
			left := predicate.Arguments[0].Name + "__GoDL_RIGHT__"
			right := predicate.Arguments[1].Name
			tbox.relation.SetSubClassOf(left, right)
		case "Declaration":
		default:
			n := ontology.Arguments[i].Name
			if _, ok := tbox.todo[n]; ok {
				tbox.todo[n]++
			} else {
				tbox.todo[n] = 1
			}
		}
	}

	tbox.relation.ComputeAll()

	for n, v := range tbox.todo {
		log.Println("Warning,", n, "not implemented ("+strconv.Itoa(v), "occurences)")
	}

	// create tables
	log.Println("creating tables...")
	db := _properties.db

	for _, class := range tbox.classes {
		_properties.classNames = append(_properties.classNames, class)

		request := fmt.Sprintf(`CREATE TABLE '%s'
			(value TEXT, positive INTEGER, weight FLOAT, origin TEXT, PRIMARY KEY (value, positive, weight, origin))`,
			class)
		if _, err := db.Exec(request); err != nil {
			log.Println(request)
			log.Println(err)
		}
	}

	for _, objectProperty := range tbox.objectProperties {
		_properties.objectPropertyNames = append(_properties.objectPropertyNames, objectProperty)

		request := fmt.Sprintf("CREATE TABLE '%s' (leftValue TEXT, rightValue TEXT, positive INTEGER, weight FLOAT, origin TEXT, PRIMARY KEY (leftValue, rightValue, positive, weight, origin))", objectProperty)
		if _, err := db.Exec(request); err != nil {
			log.Println("Warning:", err)
		}
	}

	if tbox.pass > 0 {
		log.Println("Warning:", tbox.pass, "passed erguments...")
	}

	log.Println("saving TBox...")
	saveTBox()

	return ontology != nil
}

func saveTBox() {
	val, _ := tbox.relation.JSON()
	requestJSON := fmt.Sprintf("INSERT INTO  __GoDL_JSON__ VALUES ('TBox', '%s');", val)
	if _, err := _properties.db.Exec(requestJSON); err != nil {
		log.Fatal(err)
	}
}

func saveOrigins() {
	val, _ := json.Marshal(_properties.aboxes)
	requestJSON := fmt.Sprintf("INSERT INTO  __GoDL_JSON__ VALUES ('origins', '%s');", val)
	if _, err := _properties.db.Exec(requestJSON); err != nil {
		log.Fatal(err)
	}
}

func saveNames() {
	val, _ := json.Marshal(_properties.classNames)
	requestJSON := fmt.Sprintf("INSERT INTO  __GoDL_JSON__ VALUES ('classNames', '%s');", val)
	if _, err := _properties.db.Exec(requestJSON); err != nil {
		log.Fatal(err)
	}

	val, _ = json.Marshal(_properties.objectPropertyNames)
	requestJSON = fmt.Sprintf("INSERT INTO  __GoDL_JSON__ VALUES ('objectPropertyNames', '%s');", val)
	if _, err := _properties.db.Exec(requestJSON); err != nil {
		log.Fatal(err)

	}

}

func randomGenerator(n int) float64 {
	return 1 - rand.Float64()
}

func constantGenerator(n int) float64 {
	return 1
}

func nanGenerator(n int) float64 {
	return math.NaN()
}

func decreasingGenerator(n int) float64 {
	return 1 / (math.Log(float64(n-1) + math.E))
}

func increasingGenerator(n int) float64 {
	return 1 - 1/(math.Log(float64(n-1)+math.E+0.00000000001))
}

func parseFlags() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "godl-import\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  godl-import [arguments] TBox ABox1 ABox2...\n\n")
		fmt.Fprintf(os.Stderr, "arguments:\n")

		flag.PrintDefaults()
	}

	var dbname string
	flag.StringVar(&dbname, "d", "", "database filename")

	var help bool
	flag.BoolVar(&help, "h", false, "this message")

	var version bool
	flag.BoolVar(&version, "v", false, "version")

	flag.BoolVar(&_properties.doNotImportTBox, "n", false, "do not import the TBox")

	flag.BoolVar(&_properties.Debug, "g", false, "add some debug output")

	var computeWeigthMethod int
	flag.IntVar(&computeWeigthMethod, "w", 0, "compute Weigths (0: all 1, 1: random, 2: decreasing order, 3: increasing number, 4: all NaN)")

	flag.Parse()

	args := flag.Args()

	if version {
		fmt.Println("godl-query version:", Version)
		os.Exit(0)
	}

	if len(args) == 0 || help {
		flag.Usage()
		os.Exit(1)
	}

	if dbname != "" {
		_properties.dbname = dbname
		_properties.fullname = _properties.dirname + string(os.PathSeparator) + _properties.dbname
		fmt.Println(_properties.fullname)
	}

	switch computeWeigthMethod {
	case 0:
		_properties.weightGenerator = constantGenerator
	case 1:
		_properties.weightGenerator = randomGenerator
	case 2:
		_properties.weightGenerator = decreasingGenerator
	case 3:
		_properties.weightGenerator = increasingGenerator
	case 4:
		_properties.weightGenerator = nanGenerator
	default:
		_properties.weightGenerator = constantGenerator
	}

	_properties.tbox = flag.Arg(0)

	for i := 1; i < flag.NArg(); i++ {
		_properties.aboxes = append(_properties.aboxes, flag.Arg(i))
	}
}

func init() {
	usr, _ := user.Current()
	_properties.dirname = usr.HomeDir + "/" + "GoDL"
	_properties.dbname = "noname.sqlite3"
	_properties.fullname = _properties.dirname + string(os.PathSeparator) + _properties.dbname
	_properties.aboxes = make([]string, 0)
	_properties.classNames = make([]string, 0)
	_properties.objectPropertyNames = make([]string, 0)
}

func main() {
	log.Println("starting GoDL...")

	parseFlags()
	defer closeDB()

	createDirectory()

	if !_properties.doNotImportTBox {
		destroyDB()
	}

	createDB()

	if !_properties.doNotImportTBox {
		importTBox()
	}

	importABoxes()

	log.Println("saving origins...")
	saveOrigins()

	log.Println("saving names...")
	saveNames()
}
