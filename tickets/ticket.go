package tickets

import "time"

type ITH struct {
	State      string    `json:"state"`
	Time       string    `json:"time"`
	ISODate    time.Time `json:"isodate"`
	ModifiedBy string    `json:"modifiedby"`
	Activity   string    `json:"activity"`
}
type TicketLog struct {
	Date    string    `json:"date"`
	ISODate time.Time `json:"isodate"`
	Info    string    `json:"info"`
	User    string    `json:"user"`
}
type TicketParent struct {
	Status     string    `json:"status"`
	Sev        string    `json:"sev"`
	Number     string    `json:"number"`
	Created    string    `json:"created"`
	ISOCreated time.Time `json:"isocreted"`
}

type Ticket struct {
	Sev             string       `json:"sev"`
	SubrootCause    string       `json:"subroot_cause"`
	Opened          string       `json:"opened"`
	ISOOpened       time.Time    `json:"isoopened"`
	Parent          TicketParent `json:"parent"`
	Handover        string       `json:"handover"`
	Abstract        string       `json:"abstract"`
	Number          string       `json:"number"`
	LastModifiedBy  string       `json:"lastmodifiedby"`
	State           string       `json:"state"`
	LastModified    string       `json:"lastmodified"`
	ISOLastModified time.Time    `json:"isolastmodified"`
	Role            string       `json:"role"`
	Dispatch        string       `json:"dispatch"`
	ISOClosed       time.Time    `json:isoclosed`
	Closed          string       `json:"closed"`
	Owner           string       `json:"owner"`
	RootCause       string       `json:"rootcause"`
	Restored        string       `json:"restored"`
	Ith             []ITH        `json:"ith"`
	Logs            []TicketLog  `json:"logs"`
}
