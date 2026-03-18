# Hook Protocol Tests

# =============================================================================
section "Hook Protocol Tests"
# =============================================================================
# Claude protocol
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"git reset --hard"}}' \
  'Claude protocol: git reset --hard'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"rm -rf /"}}' \
  'Claude protocol: rm -rf /'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"terraform destroy"}}' \
  'Claude protocol: terraform destroy'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"DROP DATABASE production"}}' \
  'Claude protocol: DROP DATABASE'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"kubectl delete namespace prod"}}' \
  'Claude protocol: kubectl delete namespace'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"aws ec2 terminate-instances --instance-ids i-123"}}' \
  'Claude protocol: aws ec2 terminate'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"docker system prune -af"}}' \
  'Claude protocol: docker system prune'

expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":"git status"}}' \
  'Claude protocol: git status (safe)'
expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":"ls -la"}}' \
  'Claude protocol: ls -la (safe)'
expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":"terraform plan"}}' \
  'Claude protocol: terraform plan (safe)'
expect_hook_allow \
  '{"toolName":"Read","toolInput":{"path":"file.txt"}}' \
  'Claude protocol: non-Bash tool (pass through)'

# Copilot protocol
expect_hook_deny \
  '{"toolName":"launch-process","toolArgs":{"command":"rm -rf /"}}' \
  'Copilot protocol: rm -rf /'
expect_hook_deny \
  '{"toolName":"launch-process","toolArgs":{"command":"kubectl delete namespace prod"}}' \
  'Copilot protocol: kubectl delete namespace'
expect_hook_deny \
  '{"toolName":"launch-process","toolArgs":{"command":"terraform destroy"}}' \
  'Copilot protocol: terraform destroy'
expect_hook_allow \
  '{"toolName":"launch-process","toolArgs":{"command":"ls -la"}}' \
  'Copilot protocol: ls -la (safe)'
expect_hook_allow \
  '{"toolName":"launch-process","toolArgs":{"command":"git status"}}' \
  'Copilot protocol: git status (safe)'

# Edge cases
expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":""}}' \
  'Hook: empty command'
