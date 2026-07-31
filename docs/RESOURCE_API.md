# Resource API

`Resource` is Runspace's workspace-level abstraction for a source of files and
tools. A resource can be a remote Git repository, a local Git mirror, or a plain
host folder. Git-specific operations may still use repository terminology
internally.

The canonical HTTP routes are:

```text
GET|POST /workspaces/{workspaceID}/resources
POST     /workspaces/{workspaceID}/resources/{resourceID}/clone
GET      /workspaces/{workspaceID}/resources/{resourceID}/tree
GET      /workspaces/{workspaceID}/resources/{resourceID}/file
GET      /workspaces/{workspaceID}/resources/{resourceID}/changes
GET      /workspaces/{workspaceID}/resources/{resourceID}/diff
GET|POST /workspaces/{workspaceID}/resources/{resourceID}/sync
GET      /workspaces/{workspaceID}/resources/{resourceID}/terminal
```

Channels accept and return `resource_id` and `resource_ids`. During the
migration, responses also include `repository_id` and `repository_ids`, and the
old `/repositories/...` routes remain compatibility aliases. Existing database
tables are intentionally unchanged so current workspace data requires no
destructive migration.
