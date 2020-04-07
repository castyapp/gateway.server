package hub

var (
	UsersHub    *UserHub
	TheatersHub *TheaterHub
)

func init() {
	UsersHub    = NewUserHub()
	TheatersHub = NewTheaterHub(UsersHub)
}