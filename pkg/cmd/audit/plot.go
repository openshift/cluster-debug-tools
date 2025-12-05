package audit

import (
	"fmt"
	"os"
	"path/filepath"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type plotWriter struct {
	to string
}

func (w plotWriter) Write(events []*auditv1.Event) error {
	if len(events) == 0 {
		return nil
	}

	to := w.to
	if len(to) == 0 {
		tmpDir, err := os.MkdirTemp("", "gnuplot-")
		if err != nil {
			return fmt.Errorf("error creating temporary directory: %w", err)
		}
		to = tmpDir
	}

	dat, err := os.OpenFile(filepath.Join(to, "output.dat"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open data file to write: %w", err)
	}

	const dateFmt = "2006-01-02 15:04:05.000"
	var yMax int64
	func() {
		defer dat.Close()
		for _, event := range events {
			duration := event.StageTimestamp.Time.Sub(event.RequestReceivedTimestamp.Time).Milliseconds()
			if duration <= 0 {
				continue
			}
			if duration > yMax {
				yMax = duration
			}
			from := event.RequestReceivedTimestamp.Time.Format(dateFmt)
			fmt.Fprintf(dat, "%s,%d\n", from, duration)
		}
	}()

	gp, err := os.OpenFile(filepath.Join(to, "output.gp"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open command file to write: %w", err)
	}
	defer gp.Close()

	fmt.Fprintf(gp, "set title \"latency distribution over time\" font \",16\"\n")
	fmt.Fprintf(gp, "set xlabel \"time\"\n")
	fmt.Fprintf(gp, "set ylabel \"latency im milliseconds\"\n")
	fmt.Fprintf(gp, "set grid\n")
	fmt.Fprintf(gp, "set datafile separator \",\"\n")
	fmt.Fprintf(gp, "set xdata time\n")
	fmt.Fprintf(gp, "set format x \"%%Y-%%m-%%d\\n%%H:%%M:%%S\"\n")
	fmt.Fprintf(gp, "set timefmt \"%%Y-%%m-%%d %%H:%%M:%%S\"\n")

	xMin := events[0].RequestReceivedTimestamp.Time.Format(dateFmt)
	xMax := events[len(events)-1].RequestReceivedTimestamp.Time.Format(dateFmt)
	fmt.Fprintf(gp, "set xrange [\"%s\":\"%s\"]\n", xMin, xMax)
	fmt.Fprintf(gp, "set yrange [0:%d]\n", yMax)
	return nil
}
