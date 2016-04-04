package godl

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

func Test(t *testing.T) {
	var elements = [...]string{"a", "b", "c", "d", "e", "f", "g"}
	r := NewRelation(len(elements))

	for _, e := range elements {
		r.AddElement(e)
	}

	r.SetSubClassOf("c", "a")
	r.SetSubClassOf("d", "a")
	r.SetDisjointClasses("c", "d")

	r.SetSubClassOf("b", "c")
	r.SetSubClassOf("e", "b")
	r.SetSubClassOf("f", "d")
	r.SetSubClassOf("b", "g")
	r.SetSubClassOf("g", "b")

	r.Print()
	r.ComputeAll()
	r.Print()

	fmt.Println()

	j, _ := r.JSON()

	fmt.Println(string(j))

	fmt.Println()

}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
