package pruebas

import (
	"fmt"
	"sync"
	"time"

	domain "github.com/Cristto/surebets-live/internal"
)

func main() {
	dataBet := domain.NewDataBet()
	channelSurebet := make(chan domain.Surebet, 100)
	buffer := make(chan string, 100)

	var wg sync.WaitGroup

	testEntries := []string{
		"bet:Girona:Las Palmas:tuo:2,5:Mas:5.50",
		"win:Girona:Las Palmas:tuo:3,5:menos de:1.25",

		"bwi:Girona:Las Palmas:tuo:3,5:más de:12.50",
		"mar:Girona:Las Palmas:tuo:4,5:Menos:1.10",

		"pin:Girona:Las Palmas:tuo:1,5:más de:2.20",
		"bet:Girona:Las Palmas:tuo:2,5:Menos:1.30",

		"win:Girona:Las Palmas:tuo:2,5:más de:6.10",
		"bwi:Girona:Las Palmas:tuo:3,5:menos de:1.20",

		"mar:Girona:Las Palmas:tuo:0,5:más de:1.80",
		"pin:Girona:Las Palmas:tuo:1,5:menos de:1.30",
	}

	// workers
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range buffer {
				dataBet.SaveEntry(data, channelSurebet)
			}
		}()
	}

	// lector de surebets
	var surebetWG sync.WaitGroup
	surebetWG.Add(1)
	go func() {
		defer surebetWG.Done()
		cont := 0
		for surebet := range channelSurebet {
			cont++
			fmt.Printf(
				"[%d] 💰 Surebet encontrada:\n  [%s, Ident: %s, Header: %s, Cuota: %.2f]\n  [%s, Ident: %s, Header: %s, Cuota: %.2f]\n  Valor: %.4f, Ganancia: %.2f%%\n\n",
				cont,
				surebet.House, surebet.Ident, surebet.Header, surebet.Odd,
				surebet.OppositeHouse, surebet.OppositeIdent, surebet.OppositeHeader, surebet.OppositeOdd,
				surebet.Surebet, surebet.Percent,
			)
		}
	}()

	// metemos entradas manuales en el buffer
	for _, entry := range testEntries {
		buffer <- entry
	}

	// esperamos un poco para que MonitorInactivity detecte inactividad y lance cálculos
	time.Sleep(4 * time.Second)

	close(buffer)
	wg.Wait()

	// esperamos un poco más por si queda algún cálculo en marcha
	time.Sleep(1 * time.Second)

	close(channelSurebet)
	surebetWG.Wait()

	fmt.Println("\n===== DATOS GUARDADOS =====")
	dataBet.PrintData()
}
