# Solo Product Analysis: Optimization & Feature Suggestions

## Executive Summary

Based on comprehensive codebase analysis of the Solo AI coding assistant platform, this document identifies 10 major feature opportunities and 5 quick wins for product optimization.

---

## 10 Major Feature Opportunities

### 1. Schedule Management UI
**Status**: Backend fully implemented, frontend missing
**Location**: `app-bridge/src/server/schedule/`
**Details**:
- Full CRUD backend exists (`types.ts`, `rpc-schemas.ts`)
- Supports cron/every cadence patterns
- Can target existing agents or create new ones
- **Gap**: No frontend screen or navigation for schedule management
**Impact**: High - enables automated recurring tasks

### 2. MCP Server Configuration Interface
**Status**: Protocol-level support, no UI
**Location**: `daemon/internal/agent/provider_opencode_mcp.go`
**Details**:
- `supportsMcpServers` capability flag exists
- MCP implementation complete in daemon
- **Gap**: No user-facing MCP server management UI
**Impact**: High - enables external tool integration

### 3. Session Persistence for Draft Agents
**Status**: Explicitly disabled
**Location**: `app/src/screens/workspace/workspace-draft-agent-tab.tsx`
**Details**:
- `DRAFT_CAPABILITIES` sets `supportsSessionPersistence: false`
- **Question**: Why disabled? Could enable persistent draft conversations
**Impact**: Medium - improves UX for draft agent users

### 4. Expanded Feature Toggle System
**Status**: Partially implemented
**Location**: `app/src/components/agent-status-bar.tsx`
**Details**:
- `AgentFeature` type supports toggle/select features
- Only 2 icons mapped: `list-todo` and `zap`
- **Gap**: Many potential features unmapped
**Impact**: Medium - enables richer agent customization

### 5. Multi-Remote Project Support
**Status**: GitHub-only
**Location**: `app/src/screens/projects/projects-screen.tsx`
**Details**:
- Explicit message: "Non-GitHub remote projects aren't supported yet"
- **Gap**: GitLab, Bitbucket, Azure DevOps support
**Impact**: Medium - expands user base

### 6. Enhanced Permission Request UX
**Status**: Basic implementation exists
**Location**: `app-bridge/src/server/agent/agent-sdk-types.ts`
**Details**:
- 5 permission kinds: `tool | plan | question | mode | other`
- Allow/deny responses
- **Gap**: Could add granular permissions, time-limited grants, bulk actions
**Impact**: Medium - improves security UX

### 7. Dynamic Agent Mode Switching
**Status**: Flag exists, implementation unclear
**Location**: `app-bridge/src/server/agent/agent-sdk-types.ts`
**Details**:
- `supportsDynamicModes` capability flag
- **Gap**: UI for real-time mode switching mid-conversation
**Impact**: Medium - enables adaptive agent behavior

### 8. Streaming Status Indicators
**Status**: Flags exist, UI could enhance
**Location**: `protocol/message_common.go`
**Details**:
- `supportsStreaming` and `supportsReasoningStream` flags
- **Gap**: Visual indicators for streaming vs non-streaming agents
**Impact**: Low-Medium - improves transparency

### 9. Tool Invocation Visualization
**Status**: Flag exists, UI minimal
**Location**: `protocol/message_common.go`
**Details**:
- `supportsToolInvocations` capability
- **Gap**: Rich UI for viewing tool calls, parameters, results
**Impact**: Medium - improves debugging and transparency

### 10. Workspace Templates
**Status**: New workspace screen exists
**Location**: `app/src/screens/workspace/`
**Details**:
- Can create new workspaces
- **Gap**: Pre-configured templates (e.g., "Web Dev", "Data Science", "Mobile")
**Impact**: Medium - accelerates onboarding

---

## 5 Quick Wins

### 1. Add More Feature Toggle Icons
**Effort**: Low
**Action**: Map additional icons in `agent-status-bar.tsx`
**Suggested icons**:
- `brain` for reasoning/thinking
- `wrench` for tools
- `calendar` for scheduling
- `shield` for permissions
- `database` for MCP

### 2. Enable Session Persistence for Drafts
**Effort**: Low
**Action**: Change `supportsSessionPersistence: true` in `DRAFT_CAPABILITIES`
**Risk**: Need to verify backend support

### 3. Add Schedule to Navigation
**Effort**: Low-Medium
**Action**: Add schedule screen route and nav item
**Dependencies**: Reuse existing backend endpoints

### 4. Add MCP Settings Section
**Effort**: Medium
**Action**: Add MCP server management in settings screen
**Dependencies**: May need new RPC endpoints

### 5. Improve Permission Request Categorization
**Effort**: Low
**Action**: Group permissions by type, add icons
**Location**: Permission request/response UI

---

## Implementation Priority Matrix

| Feature | Impact | Effort | Priority |
|---------|--------|--------|----------|
| Schedule Management UI | High | Medium | **P1** |
| MCP Server Configuration | High | Medium | **P1** |
| Session Persistence | Medium | Low | **P2** |
| Feature Toggle Expansion | Medium | Low | **P2** |
| Multi-Remote Support | Medium | High | **P3** |
| Enhanced Permissions | Medium | Medium | **P2** |
| Dynamic Mode Switching | Medium | Medium | **P3** |
| Streaming Indicators | Low-Medium | Low | **P3** |
| Tool Invocation UI | Medium | Medium | **P2** |
| Workspace Templates | Medium | Medium | **P3** |

---

## Technical Context

### Key Files Referenced
- `app/src/screens/dashboard/dashboard-screen.tsx` - Main dashboard
- `app/src/screens/settings/providers-section.tsx` - Provider management
- `app/src/components/agent-status-bar.tsx` - Model/mode/feature selector
- `app/src/screens/workspace/workspace-draft-agent-tab.tsx` - Draft agent capabilities
- `app-bridge/src/server/schedule/types.ts` - Schedule backend types
- `app-bridge/src/server/schedule/rpc-schemas.ts` - Schedule RPC endpoints
- `app-bridge/src/server/agent/agent-sdk-types.ts` - Core agent types
- `daemon/internal/agent/provider_opencode_mcp.go` - MCP implementation
- `protocol/message_common.go` - Capability flags

### Agent Capability Flags
```typescript
supportsStreaming: boolean
supportsSessionPersistence: boolean
supportsDynamicModes: boolean
supportsMcpServers: boolean
supportsReasoningStream: boolean
supportsToolInvocations: boolean
```

### Permission Types
```typescript
type PermissionKind = 'tool' | 'plan' | 'question' | 'mode' | 'other'
```

---

*Analysis completed: 2026-06-01*
*Scope: Frontend screens, components, backend capabilities, protocol definitions*
