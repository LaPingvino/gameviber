package gml

// The syntax tree: one for every GML version (docs/DESIGN.md §1).

// Node is any part of the tree: where it starts.
type Node interface{ At() Pos }

// Expr is an expression.
type Expr interface {
	Node
	expr()
}

// Stmt is a statement.
type Stmt interface {
	Node
	stmt()
}

type (
	Num struct {
		P    Pos
		Text string
	} // 12, 1.5, $FF (as written)
	Str struct {
		P     Pos
		Value string
	} // a string's text, without quotes
	Name struct {
		P    Pos
		Name string
	} // a variable, object, resource or constant
	Field struct {
		P    Pos
		X    Expr
		Name string
	} // X.Name: other.x, global.score, obj.speed
	Index struct {
		P Pos
		X Expr
		I []Expr
	} // X[i] or X[i, j]
	Call struct {
		P    Pos
		Fn   string
		Args []Expr
	}
	Unary struct {
		P  Pos
		Op string
		X  Expr
	} // ! not - + ~
	Binary struct {
		P    Pos
		Op   string
		L, R Expr
	} // "=" between expressions is a comparison
)

type (
	Block struct {
		P    Pos
		List []Stmt
	} // { … } or begin … end
	Var struct {
		P      Pos
		Global bool
		Names  []string
	} // var a, b; / globalvar a;
	Assign struct {
		P      Pos
		Target Expr
		Op     string
		Value  Expr
	} // = := += -= *= /= &= |= ^= <<= >>=
	ExprStmt struct {
		P Pos
		X Expr
	} // a call
	If struct {
		P          Pos
		Cond       Expr
		Then, Else Stmt
	} // Else nil when absent
	While struct {
		P    Pos
		Cond Expr
		Body Stmt
	}
	Do struct {
		P     Pos
		Body  Stmt
		Until Expr
	}
	Repeat struct {
		P     Pos
		Count Expr
		Body  Stmt
	}
	For struct {
		P    Pos
		Init Stmt
		Cond Expr
		Step Stmt
		Body Stmt
	}
	With struct {
		P      Pos
		Target Expr
		Body   Stmt
	}
	Switch struct {
		P     Pos
		X     Expr
		Cases []Case
	}
	Exit     struct{ P Pos }
	Break    struct{ P Pos }
	Continue struct{ P Pos }
	Return   struct {
		P Pos
		X Expr
	}
)

// Case is one case of a switch: its values (nil for default) and the
// statements up to the next case.
type Case struct {
	P      Pos
	Values []Expr
	Body   []Stmt
}

func (n *Num) At() Pos      { return n.P }
func (n *Str) At() Pos      { return n.P }
func (n *Name) At() Pos     { return n.P }
func (n *Field) At() Pos    { return n.P }
func (n *Index) At() Pos    { return n.P }
func (n *Call) At() Pos     { return n.P }
func (n *Unary) At() Pos    { return n.P }
func (n *Binary) At() Pos   { return n.P }
func (n *Block) At() Pos    { return n.P }
func (n *Var) At() Pos      { return n.P }
func (n *Assign) At() Pos   { return n.P }
func (n *ExprStmt) At() Pos { return n.P }
func (n *If) At() Pos       { return n.P }
func (n *While) At() Pos    { return n.P }
func (n *Do) At() Pos       { return n.P }
func (n *Repeat) At() Pos   { return n.P }
func (n *For) At() Pos      { return n.P }
func (n *With) At() Pos     { return n.P }
func (n *Switch) At() Pos   { return n.P }
func (n *Exit) At() Pos     { return n.P }
func (n *Break) At() Pos    { return n.P }
func (n *Continue) At() Pos { return n.P }
func (n *Return) At() Pos   { return n.P }

func (*Num) expr()    {}
func (*Str) expr()    {}
func (*Name) expr()   {}
func (*Field) expr()  {}
func (*Index) expr()  {}
func (*Call) expr()   {}
func (*Unary) expr()  {}
func (*Binary) expr() {}

func (*Block) stmt()    {}
func (*Var) stmt()      {}
func (*Assign) stmt()   {}
func (*ExprStmt) stmt() {}
func (*If) stmt()       {}
func (*While) stmt()    {}
func (*Do) stmt()       {}
func (*Repeat) stmt()   {}
func (*For) stmt()      {}
func (*With) stmt()     {}
func (*Switch) stmt()   {}
func (*Exit) stmt()     {}
func (*Break) stmt()    {}
func (*Continue) stmt() {}
func (*Return) stmt()   {}
