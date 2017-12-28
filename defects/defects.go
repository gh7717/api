package defect

type Defect struct {
	Id   string   `bson:"_id"`
	Info []string `json:"info"`
}
type DefectOutput struct {
	Number  string `json:"number"`
	Defects string `json:"defect"`
}
