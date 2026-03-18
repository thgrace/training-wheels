# AI Tools

# =============================================================================
section "AI Tools"
# =============================================================================
expect_deny 'huggingface-cli delete-cache'             'hf delete-cache'
expect_deny 'wandb sweep stop abc123'                  'wandb sweep stop'
expect_deny 'wandb artifact rm entity/project/artifact' \
                                                       'wandb artifact rm'
