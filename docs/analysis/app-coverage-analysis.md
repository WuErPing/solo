# App Coverage Analysis

> Generated: 2026-05-28  
> Tool: Vitest with v8 provider  
> Overall Coverage: **35.51%** (29,467 / 82,979 lines)

## Summary

React Native/Expo 应用的测试覆盖率为 35.51%，主要集中在工具函数、状态管理和键盘处理等核心逻辑层。UI 组件层（components、screens）覆盖率较低，这是前端应用的典型特征。

## Coverage by Directory

### High Coverage (>75%)

| Directory | Coverage | Lines | Status |
|-----------|----------|-------|--------|
| `src/query` | **100.0%** | 12 | ✅ Complete |
| `src/styles` | **91.8%** | 861 | ✅ Excellent |
| `src/keyboard` | **87.8%** | 1,418 | ✅ Excellent |
| `src/constants` | **79.3%** | 82 | ✅ Good |
| `src/utils` | **79.3%** | 5,241 | ✅ Good |
| `src/terminal` | **75.9%** | 1,076 | ✅ Good |
| `src/test` | **74.2%** | 31 | ✅ Good |
| `src/types` | **73.8%** | 1,173 | ✅ Good |

### Medium Coverage (40-75%)

| Directory | Coverage | Lines | Status |
|-----------|----------|-------|--------|
| `src/stores` | **65.4%** | 5,697 | 🟡 Zustand stores need more tests |
| `src/runtime` | **65.0%** | 1,835 | 🟡 Runtime logic partially covered |
| `src/panels` | **44.5%** | 1,961 | 🟡 Panel components under-tested |
| `src/attachments` | **44.1%** | 696 | 🟡 Attachment handling needs tests |
| `src/desktop` | **43.7%** | 2,708 | 🟡 Desktop-specific code gaps |
| `src/hooks` | **43.3%** | 7,594 | 🟡 Custom hooks under-tested |

### Low Coverage (<40%)

| Directory | Coverage | Lines | Status |
|-----------|----------|-------|--------|
| `src/screens` | **31.4%** | 11,102 | 🔴 Screen components largely untested |
| `src/contexts` | **25.7%** | 2,707 | 🔴 React contexts need tests |
| `src/components` | **18.7%** | 35,434 | 🔴 UI components minimally tested |
| `src/app` | **7.3%** | 1,676 | 🔴 App entry/navigation barely covered |

### Zero Coverage

| Directory | Lines | Reason |
|-----------|-------|--------|
| `android` | 1,490 | Native platform code |
| `scripts` | 97 | Build scripts |
| `src/polyfills` | 64 | Polyfill code |
| `src/lib` | 16 | Library code |
| `.expo` | 1 | Generated config |
| `src/voice` | 0 | Empty directory |

## Analysis

### Strengths

1. **工具函数覆盖良好**: `src/utils` (79.3%) 和 `src/keyboard` (87.8%) 的高覆盖率确保了核心逻辑的可靠性
2. **状态管理基本覆盖**: `src/stores` (65.4%) 覆盖了主要的 Zustand stores
3. **类型定义完整**: `src/types` (73.8%) 确保了类型安全
4. **样式系统完整**: `src/styles` (91.8%) 主题和样式逻辑经过充分测试

### Critical Gaps

1. **UI 组件测试不足** (18.7%):
   - 35,434 行代码中仅 6,621 行被测试覆盖
   - 包括按钮、输入框、列表等基础组件
   - **风险**: UI 回归风险高，视觉和交互问题难以自动发现

2. **屏幕组件缺失** (31.4%):
   - 11,102 行中仅 3,483 行覆盖
   - 主要业务页面（agent、chat、settings）测试不足
   - **风险**: 用户流程中断、状态管理错误

3. **React Context 未测试** (25.7%):
   - 2,707 行中仅 695 行覆盖
   - 全局状态（theme、auth、workspace）缺乏测试
   - **风险**: 跨组件状态同步问题

4. **自定义 Hooks 覆盖不足** (43.3%):
   - 7,594 行中仅 3,289 行覆盖
   - 包括 `useAgent`、`useWorkspace`、`useTerminal` 等核心 hooks
   - **风险**: Hook 逻辑错误会影响多个组件

### Root Causes

1. **UI 测试复杂性**: React Native 组件需要模拟原生环境，测试设置成本高
2. **集成测试缺失**: 主要依赖单元测试，缺少组件集成测试
3. **E2E 测试独立**: Playwright E2E 测试不在覆盖率统计内
4. **历史债务**: 早期快速开发阶段未建立测试文化

## Priority Recommendations

### P0: Critical Business Logic

1. **核心 Hooks 测试** (目标: 70%+)
   - `useAgent`: agent 生命周期管理
   - `useWorkspace`: workspace 状态和操作
   - `useTerminal`: terminal 连接和数据流
   - `useAuth`: 认证流程

2. **Context Provider 测试** (目标: 60%+)
   - `ThemeProvider`: 主题切换逻辑
   - `AuthProvider`: 登录/登出状态
   - `WorkspaceProvider`: workspace 上下文

3. **关键 Screen 组件** (目标: 50%+)
   - `AgentScreen`: agent 交互主界面
   - `ChatScreen`: 聊天界面
   - `SettingsScreen`: 配置界面

### P1: Common Components

1. **基础 UI 组件** (目标: 40%+)
   - `Button`, `Input`, `Card` 等基础组件
   - `List`, `Modal`, `Toast` 等交互组件

2. **业务组件** (目标: 50%+)
   - `AgentCard`: agent 展示组件
   - `ChatMessage`: 消息渲染
   - `TerminalView`: terminal 显示

### P2: Edge Cases & Integration

1. **错误边界测试**
   - Error boundary 组件
   - 网络错误处理
   - 异常状态恢复

2. **集成测试**
   - 用户流程测试（创建 agent → 发送消息 → 查看响应）
   - 跨组件状态同步

## Testing Strategy

### Current Approach
- **Unit Tests**: Vitest + React Native Testing Library
- **E2E Tests**: Playwright (独立运行，不计入覆盖率)
- **Coverage Tool**: v8 provider

### Recommended Improvements

1. **增加组件测试工具**:
   ```typescript
   // 统一的 render wrapper
   import { renderWithProviders } from '@/test/utils';
   
   test('AgentCard displays agent info', () => {
     renderWithProviders(<AgentCard agent={mockAgent} />);
     expect(screen.getByText('Test Agent')).toBeInTheDocument();
   });
   ```

2. **Hook 测试模式**:
   ```typescript
   import { renderHook } from '@testing-library/react-hooks';
   
   test('useAgent loads agent data', async () => {
     const { result, waitForNextUpdate } = renderHook(() => useAgent('agent-123'));
     await waitForNextUpdate();
     expect(result.current.agent).toBeDefined();
   });
   ```

3. **Mock 策略**:
   - Native modules: `jest.mock('expo-clipboard')`
   - Network: MSW (Mock Service Worker)
   - Navigation: `@react-navigation/testing`

## Comparison with Backend

| Layer | Backend (Go) | App (TypeScript) |
|-------|--------------|------------------|
| Utils/Helpers | 79-92% | 79.3% |
| Business Logic | 65-88% | 43-65% |
| UI/Components | N/A | 18-31% |
| Overall | ~62% | 35.5% |

**Key Difference**: 后端代码主要是业务逻辑，易于单元测试；前端包含大量 UI 代码，测试成本更高。

## Next Steps

1. **短期 (1-2 weeks)**:
   - 为核心 hooks 添加测试 (useAgent, useWorkspace, useTerminal)
   - 测试关键 context providers
   - 覆盖主要 screen 组件的 happy path

2. **中期 (1 month)**:
   - 建立组件测试模式 (renderWithProviders)
   - 添加常见 UI 组件测试
   - 集成 MSW 进行 API mock

3. **长期 (quarter)**:
   - 达到 50% 整体覆盖率
   - 建立 visual regression testing
   - 自动化 accessibility 测试

## Notes

- 覆盖率数字不包括 E2E 测试（Playwright）
- Native modules (android/, ios/) 不在测试范围内
- 部分 0% 覆盖的目录是生成代码或配置（.expo, scripts）
- `src/voice` 目录为空，可考虑删除或实现
