package authorize

type Request struct {
	LocationId string `json:"location_id"`
	IdTag      string `json:"id_tag"`
}
