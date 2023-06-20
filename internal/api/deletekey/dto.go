// Package deletekey defines delete API key handler.
package deletekey

// input represents the request body.
type input struct {
	Hash string `uri:"id" binding:"required"` // Key ID.
}

// workflowInput represents the workflow input.
type workflowInput struct {
	Hash string
}

// workflowOutput represents the workflow output.
type workflowOutput struct {
	Hash    string
}
