package types

type Cluster struct {
    Name string `json:"name"`
} 

type User struct {
    Username string
    Token    string
    Role     string
} 


type Rule struct {
	RuleFile string
	Content  string
	Enable   bool
}