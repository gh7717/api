package user

type User struct {
	Name      string `json:"name"`
	Is_Active bool   `json:"is_active"`
	Real_Name string `json:"real_name"`
	Current   bool   `json:"current"`
	Is_Admin  bool   `json:"is_admin"`
	ID        string `json:"id"`
	Engineer  bool   `json:"engineer"`
	Attuid    string `json:"attuid"`
}
