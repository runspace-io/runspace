package collaboration

import "strings"

func inheritedRepositoryIDs(ids []string, parent Channel, hasParent bool) []string {
	ids = normalizeRepositoryIDs(ids)
	if len(ids) > 0 || !hasParent {
		return ids
	}
	ids = append(ids, parent.RepositoryIDs...)
	if len(ids) == 0 && parent.RepositoryID != "" {
		ids = append(ids, parent.RepositoryID)
	}
	return normalizeRepositoryIDs(ids)
}

func normalizeRepositoryIDs(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func applyRepositoryPatch(channel *Channel, patch ChannelPatch) {
	if patch.RepositoryID != nil {
		setChannelRepositoryIDs(channel, []string{strings.TrimSpace(*patch.RepositoryID)})
	}
	if patch.RepositoryIDs != nil {
		setChannelRepositoryIDs(channel, *patch.RepositoryIDs)
	}
	if patch.ResourceID != nil {
		setChannelRepositoryIDs(channel, []string{strings.TrimSpace(*patch.ResourceID)})
	}
	if patch.ResourceIDs != nil {
		setChannelRepositoryIDs(channel, *patch.ResourceIDs)
	}
}

func setChannelRepositoryIDs(channel *Channel, ids []string) {
	channel.RepositoryIDs = normalizeRepositoryIDs(ids)
	channel.RepositoryID = ""
	if len(channel.RepositoryIDs) > 0 {
		channel.RepositoryID = channel.RepositoryIDs[0]
	}
}
