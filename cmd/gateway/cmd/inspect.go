package cmd

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Inspect gateway BoltDB",
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbInspectCmd)
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbGetCmd)
}




var dbPath string

var dbInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect BoltDB contents",
	RunE: func(cmd *cobra.Command, args []string) error {

		db, err := bolt.Open(dbPath, 0444, &bolt.Options{
			ReadOnly: true,
		})
		if err != nil {
			return err
		}
		defer db.Close()

		return db.View(func(tx *bolt.Tx) error {

			return tx.ForEach(func(name []byte, b *bolt.Bucket) error {

				fmt.Printf("\nBucket: %s\n", name)
				fmt.Println("--------------------------------")

				return b.ForEach(func(k, v []byte) error {

					fmt.Printf("%s (%d bytes)\n", k, len(v))

					return nil
				})
			})

		})

	},
}

func init() {
	dbInspectCmd.Flags().StringVar(
		&dbPath,
		"db",
		"/home/johnnyash/.ashrix/gateway/data/gateway.db",
		"BoltDB path",
	)
}


var bucket string

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List keys inside a bucket",
	RunE: func(cmd *cobra.Command, args []string) error {

		db, err := bolt.Open(dbPath, 0444, &bolt.Options{
			ReadOnly: true,
		})
		if err != nil {
			return err
		}
		defer db.Close()

		return db.View(func(tx *bolt.Tx) error {

			b := tx.Bucket([]byte(bucket))
			if b == nil {
				return fmt.Errorf("bucket %q not found", bucket)
			}

			return b.ForEach(func(k, _ []byte) error {

				fmt.Println(string(k))

				return nil
			})
		})

	},
}

func init() {
	dbListCmd.Flags().StringVar(&bucket, "bucket", "", "Bucket name")
	dbListCmd.MarkFlagRequired("bucket")
}

var key string

var dbGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show one entry",
	RunE: func(cmd *cobra.Command, args []string) error {

		db, err := bolt.Open(dbPath, 0444, &bolt.Options{
			ReadOnly: true,
		})
		if err != nil {
			return err
		}
		defer db.Close()

		return db.View(func(tx *bolt.Tx) error {

			b := tx.Bucket([]byte(bucket))
			if b == nil {
				return fmt.Errorf("bucket not found")
			}

			raw := b.Get([]byte(key))
			if raw == nil {
				return fmt.Errorf("key not found")
			}

			switch bucket {

			case "policies":

				var p pb.PolicyRule

				if err := proto.Unmarshal(raw, &p); err != nil {
					return err
				}

				fmt.Println(prototext.Format(&p))

			default:

				fmt.Printf("%s\n", raw)

			}

			return nil
		})

	},
}

func init() {
	dbGetCmd.Flags().StringVar(&bucket, "bucket", "", "Bucket")
	dbGetCmd.Flags().StringVar(&key, "key", "", "Key")

	// dbGetCmd.MarkFlagRequired("bucket")
	// dbGetCmd.MarkFlagRequired("key")
	// dbGetCmd.PersistentFlags().String("redis-url", "redis://localhost:6379/1", "Redis Connection URL")

}