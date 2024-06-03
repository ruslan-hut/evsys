package authorize

type Request struct {
	LocationId string `json:"location_id"`
	Evse       string `json:"evse_id"`
	IdTag      string `json:"id_tag"`
}
