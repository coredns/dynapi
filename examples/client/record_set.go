package main

// recordSet is the complete set of addresses for one name and DNS record type.
type recordSet struct {
	Addresses []string `json:"addresses"`
	TTL       uint32   `json:"ttl"`
}
