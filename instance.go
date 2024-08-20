
package main
import (
	"github.com/polygon-io/client-go/websocket/models"
    polygon "github.com/polygon-io/client-go/rest"
)

const apiKey = "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm"


func main() {

	// init client
	c := polygon.New(os.Getenv("POLYGON_API_KEY"))

	// set params
	params := models.GetTickerRelatedCompaniesParams{
		Ticker: "AAPL",
	}

	// make request
	res, err := c.GetTickerRelatedCompanies(context.Background(), &params)
	if err != nil {
		log.Fatal(err)
	}

	// do something with the result
	log.Print(res)

}
