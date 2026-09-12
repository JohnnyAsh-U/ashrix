package output

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/resource"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/storage"
)

func PrintResources(w io.Writer, resources []resource.Resource) {
	if w == nil {
		w = os.Stdout
	}
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tSTATUS\tDESTINATION")
	for _, r := range resources {
		dest := r.Destination
		if dest == "" && r.Port > 0 {
			dest = fmt.Sprintf(":%d", r.Port)
		}
		if dest == "" {
			dest = "-"
		}
		status := r.Status
		if status == "" {
			status = "available"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Name, r.Type, status, dest)
	}
	tw.Flush()
}

func PrintWhoami(w io.Writer, creds *storage.SessionCredentials) {
	if w == nil {
		w = os.Stdout
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE")
	if creds.UserEmail != "" {
		fmt.Fprintf(tw, "User Email\t%s\n", creds.UserEmail)
	}
	if creds.UserID != "" {
		fmt.Fprintf(tw, "User ID\t%s\n", creds.UserID)
	}
	if creds.TenantID != "" {
		fmt.Fprintf(tw, "Tenant ID\t%s\n", creds.TenantID)
	}
	fmt.Fprintf(tw, "Session Valid Until\t%s\n", creds.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
	tw.Flush()
}
