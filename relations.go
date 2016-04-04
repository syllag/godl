package godl

import (
	"fmt"
	"os"
	"sort"
)
import "log"
import "encoding/json"

// Relation is a structure representing ⊑
type Relation struct {
	Capacity               int
	Size                   int
	Elements               []string
	IndexOf                map[string]int
	IncidenceMatrix        [][]int
	CompactIncidenceMatrix [][]int
	Weights                []int
	EquivalentClasses      [][]int
	Debug                  bool
}

// NewRelation creates a new Relation of capacity capacity
func NewRelation(capacity int) *Relation {
	var r Relation
	r.Capacity = capacity
	r.Size = 0
	r.Elements = make([]string, 0, capacity)
	r.IndexOf = make(map[string]int)
	r.EquivalentClasses = make([][]int, 0)

	r.IncidenceMatrix = make([][]int, capacity)
	r.CompactIncidenceMatrix = make([][]int, capacity)
	for i := 0; i < capacity; i++ {
		r.IncidenceMatrix[i] = make([]int, capacity)
		r.CompactIncidenceMatrix[i] = make([]int, capacity)
	}

	return &r
}

func (r *Relation) ComputeEquivalentClasses() {
	marked := make([]bool, r.Size)

	for i := 0; i < r.Size; i++ {
		tmp := make([]int, 1)

		if marked[i] {
			continue
		}
		marked[i] = true
		tmp[0] = i
		for j := i + 1; j < r.Size; j++ {
			if r.IncidenceMatrix[i][j] == 1 && r.IncidenceMatrix[i][j] == r.IncidenceMatrix[j][i] {
				marked[j] = true
				tmp = append(tmp, j)
			}
		}

		r.EquivalentClasses = append(r.EquivalentClasses, tmp)
	}
}

func (r *Relation) Len() int {
	return len(r.EquivalentClasses)
}

func (r *Relation) Swap(i, j int) {
	tmp := r.EquivalentClasses[i]
	r.EquivalentClasses[i] = r.EquivalentClasses[j]
	r.EquivalentClasses[j] = tmp
}

func (r *Relation) Less(i, j int) bool {
	return r.Weights[r.EquivalentClasses[i][0]] > r.Weights[r.EquivalentClasses[j][0]]
}

func (r *Relation) SortEquivalentClasses() {
	sort.Sort(r)
}

// TODO: fix me ?
func (r *Relation) ComputeCompactIncidenceMatrix() {
	// we remove the not direct successors
	for i := 0; i < r.Size; i++ {
		for j := 0; j < r.Size; j++ {
			if val := r.IncidenceMatrix[i][j]; i != j && val != 0 {
				for k := 0; k < r.Size; k++ {
					if r.IncidenceMatrix[i][k] == 1 && r.IncidenceMatrix[k][j] == val &&
						r.IncidenceMatrix[k][i] != 1 && r.IncidenceMatrix[j][k] != 1 {
						val = 0
						break
					}
				}
				r.CompactIncidenceMatrix[i][j] = val
			}
		}

		// remove equivalent elements
		for _, eqClasses := range r.EquivalentClasses {
			for i := 1; i < len(eqClasses); i++ {
				index := eqClasses[i]
				for j := 0; j < r.Size; j++ {
					r.CompactIncidenceMatrix[index][j] = 0
					r.CompactIncidenceMatrix[j][index] = 0
				}
			}
		}

	}
}

func (r *Relation) ComputeWeights() {
	r.Weights = make([]int, r.Size)

	for i := 0; i < r.Size; i++ {
		w := 0
		for j := 0; j < r.Size; j++ {
			// if r.IncidenceMatrix[i][j] != 0 {
			if r.IncidenceMatrix[i][j] == 1 {
				w++
			}
		}

		r.Weights[i] = w
	}
}

// Print prints a reprentation of the relation
func (r *Relation) Print() {
	fmt.Println(r.IndexOf)

	for i, val := range r.Elements {
		fmt.Printf("%d: %s\n", i, val)
	}

	fmt.Println()

	fmt.Print("⊑ ")
	for i := 0; i < r.Size; i++ {
		fmt.Print(" ")
		fmt.Print(i % 10)
		fmt.Print(" ")
	}
	fmt.Println()

	for i := 0; i < r.Size; i++ {
		fmt.Printf("%d ", i%10)
		for j := 0; j < r.Size; j++ {
			val := r.IncidenceMatrix[i][j]

			switch val {
			case 0:
				fmt.Print(" . ")
			case 1:
				fmt.Print(" 1 ")
			case -1:
				fmt.Print("-1 ")
			default:
				fmt.Print(" # ")
			}
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Print("⊑ ")
	for i := 0; i < r.Size; i++ {
		fmt.Print(" ")
		fmt.Print(i % 10)
		fmt.Print(" ")
	}
	fmt.Println()
	for i := 0; i < r.Size; i++ {
		fmt.Printf("%d ", i%10)
		for j := 0; j < r.Size; j++ {
			val := r.CompactIncidenceMatrix[i][j]

			switch val {
			case 0:
				fmt.Print(" . ")
			case 1:
				fmt.Print(" 1 ")
			case -1:
				fmt.Print("-1 ")
			default:
				fmt.Print(" # ")
			}
		}
		fmt.Println()
	}
	fmt.Println()

	fmt.Println("Weights :", r.Weights)
	fmt.Println("EquivalentClasses:", r.EquivalentClasses)
	fmt.Println()

}

func (r *Relation) AddElement(s string) {
	if r.Size == r.Capacity {
		log.Fatal("Relation capacity exceeded...")
	}

	r.Elements = append(r.Elements, s)
	r.IndexOf[s] = r.Size
	r.Size++
}

// JSON return a JSON representation of the relation
func (r *Relation) JSON() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Relation) Set(predicate string, arg1 string, arg2 string) bool {
	i := r.IndexOf[arg1]
	j := r.IndexOf[arg2]

	return r.SetIndex(predicate, i, j)
}

// TODO !!!!
func (r *Relation) SetIndex(predicate string, arg1 int, arg2 int) bool {
	// TODO...

	return true
}

func (r *Relation) ComputeAll() {
	// transitivity
	for i := 0; i < r.Size; i++ {
		r.SetSubClassOf(r.Elements[i], r.Elements[i])
	}

	r.ComputeClosure()
	r.ComputeEquivalentClasses()
	r.ComputeWeights()
	r.SortEquivalentClasses()
	r.ComputeCompactIncidenceMatrix()
}

// SetSubClassOf sets the relation for SetSubClassOf
func (r *Relation) SetSubClassOf(subsumee string, subsumer string) bool {
	i := r.IndexOf[subsumee]
	j := r.IndexOf[subsumer]

	return r.SetSubClassOfIndex(i, j)
}

// SetSubClassOfIndex sets the relation for SetSubClassOf, index version
func (r *Relation) SetSubClassOfIndex(subsumee int, subsumer int) bool {
	if r.IncidenceMatrix[subsumee][subsumer] == -1 {
		return false
	}

	if r.Debug {
		fmt.Println("SetSubClassOf:", r.Elements[subsumee], r.Elements[subsumer])
	}

	r.IncidenceMatrix[subsumee][subsumer] = 1

	return true
}

func (r *Relation) SetDisjointClassesIndex(class1 int, class2 int) bool {
	if r.IncidenceMatrix[class1][class2] == 1 || r.IncidenceMatrix[class2][class1] == 1 {
		return false
	}

	if r.Debug {
		fmt.Println("SetDisjointClasses:", r.Elements[class1], r.Elements[class2])
	}

	r.IncidenceMatrix[class1][class2] = -1
	r.IncidenceMatrix[class2][class1] = -1

	return true
}

func (r *Relation) SetDisjointClasses(class1 string, class2 string) bool {
	i := r.IndexOf[class1]
	j := r.IndexOf[class2]

	return r.SetDisjointClassesIndex(i, j)
}

func (r *Relation) PrintSubClassOf(n int) {
	fmt.Print("Superclass of ", r.Elements[n], " are [ ")
	for i := 0; i < r.Size; i++ {
		if r.IncidenceMatrix[n][i] == 1 && n != i {
			fmt.Print(r.Elements[i], " ")
		}
	}
	fmt.Println("]")

	fmt.Print("Subclass of ", r.Elements[n], " are [ ")
	for i := 0; i < r.Size; i++ {
		if r.IncidenceMatrix[i][n] == 1 && n != i {
			fmt.Print(r.Elements[i], " ")
		}
	}
	fmt.Println("]")
}

func (r *Relation) PrintDisjointClassOf(n int) {
	fmt.Print("Disjoint Class of ", r.Elements[n], " are [ ")
	for i := 0; i < r.Size; i++ {
		if r.IncidenceMatrix[n][i] == -1 && n != i {
			fmt.Print(r.Elements[i], " ")
		}
	}
	fmt.Println("]")
}

// ComputeClosure computes the transitive closure of the relation (adaptation of Warshall's algorithm)
func (r *Relation) ComputeClosure() {
	m := r.IncidenceMatrix
	n := r.Size

	// transitive closure
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if m[i][j] == 0 && m[i][k] == 1 && m[k][j] == 1 {
					if i == 12 { // DEBUG
						fmt.Println("i:", i, r.Elements[i])
						fmt.Println("k:", k, r.Elements[k])
						fmt.Println("j:", j, r.Elements[j])
						fmt.Println()
					}

					m[i][j] = 1
				}
			}
		}
	}

	// negative closure
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if m[i][j] == -1 {
				for k := 0; k < n; k++ {
					if m[k][i] == 1 {

						if m[k][j] == 1 { // DEBUG
							fmt.Println("i:", i, r.Elements[i])
							fmt.Println("j:", j, r.Elements[j])
							fmt.Println("k:", k, r.Elements[k])
							fmt.Println()

							fmt.Println(r.Elements[k], "⊑ -", r.Elements[j])
							fmt.Println("because...")
							fmt.Println(r.Elements[i], "⊑ -", r.Elements[j])
							fmt.Println(r.Elements[k], "⊑", r.Elements[i])
							fmt.Println("but...")
							fmt.Println(r.Elements[k], "⊑ ", r.Elements[j])

							fmt.Println()
							r.PrintSubClassOf(i)
							r.PrintDisjointClassOf(i)
							r.PrintSubClassOf(j)
							r.PrintDisjointClassOf(j)
							r.PrintSubClassOf(k)
							r.PrintDisjointClassOf(k)
							os.Exit(-1)
						}

						m[k][j] = -1
						m[j][k] = -1
					}
				}
			}
		}
	}
}

func myAssert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}
