package workspace

// WorkspaceGitMetadata contains lightweight derived git metadata for a workspace.
type WorkspaceGitMetadata struct {
	ProjectKind        ProjectKind `json:"projectKind"`
	ProjectDisplayName string      `json:"projectDisplayName"`
	WorkspaceDisplayName string    `json:"workspaceDisplayName"`
	GitRemote          *string     `json:"gitRemote,omitempty"`
	IsWorktree         bool        `json:"isWorktree"`
	ProjectSlug        string      `json:"projectSlug"`
	RepoRoot           *string     `json:"repoRoot,omitempty"`
	CurrentBranch      *string     `json:"currentBranch,omitempty"`
	RemoteUrl          *string     `json:"remoteUrl,omitempty"`
}
