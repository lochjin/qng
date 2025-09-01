package graph

type Node interface {
	GetName() string
	GetNodeFunction() NodeFunction
}

type BaseNode struct {
	// Name is the unique identifier for the node.
	Name string

	// Function is the function associated with the node.
	// It takes a context and a slice of MessageContent as input and returns a slice of MessageContent and an error.
	Function NodeFunction
}

func NewBaseNode(name string, function NodeFunction) *BaseNode {
	return &BaseNode{Name: name, Function: function}
}

func (n *BaseNode) GetName() string {
	return n.Name
}

func (n *BaseNode) GetNodeFunction() NodeFunction {
	return n.Function
}
