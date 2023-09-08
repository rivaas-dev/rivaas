package policies

import (
	"context"
	"embed"
	"fmt"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"io/fs"
	"strings"
)

//go:embed *.rego
var policies embed.FS

func InitializeRegoFiles(ctx context.Context, c *oma.Client) error {
	err := fs.WalkDir(policies, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking policies' files: %w", err)
		}

		if !info.IsDir() && strings.HasSuffix(path, ".rego") {
			policyContent, err := fs.ReadFile(policies, path)
			if err != nil {
				return fmt.Errorf("unable to read the policy file %s) %w", path, err)
			}

			err = c.UpsertPolicy(ctx, policyContent)
			if err != nil {
				return fmt.Errorf("unable to upsert the policy %s: %w", path, err)
			}
		}

		return nil
	})

	return err
}
