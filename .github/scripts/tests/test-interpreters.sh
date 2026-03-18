# Interpreters: Python, Node.js, Ruby, Perl

# =============================================================================
section "Interpreters: Python"
# =============================================================================
expect_deny 'python -c "import shutil; shutil.rmtree(\"/var\")"' \
                                                       'python shutil.rmtree'
expect_deny 'python -c "import os; os.remove(\"/etc/passwd\")"' \
                                                       'python os.remove'
expect_deny 'python -c "import os; os.rmdir(\"/var\")"' \
                                                       'python os.rmdir'
expect_deny 'python -c "import os; os.system(\"rm -rf /\")"' \
                                                       'python os.system rm -rf'
expect_deny 'python -c "import subprocess; subprocess.run([\"rm\", \"-rf\", \"/\"])"' \
                                                       'python subprocess destructive'
expect_deny 'python -c "from pathlib import Path; import shutil; shutil.rmtree(Path(\"/var\"))"' \
                                                       'python pathlib rmtree'
expect_deny 'python -c "open(\"/etc/passwd\", \"w\").truncate()"' \
                                                       'python open write truncate'

# =============================================================================
section "Interpreters: Node.js"
# =============================================================================
expect_deny 'node -e "fs.rmSync(\"/var\", {recursive:true})"' \
                                                       'node fs.rmSync'
expect_deny 'node -e "fs.unlinkSync(\"/etc/passwd\")"' 'node fs.unlinkSync'
expect_deny 'node -e "require(\"child_process\").execSync(\"rm -rf /\")"' \
                                                       'node child_process exec'
expect_deny 'node -e "const fs = require(\"fs\"); fs.rm(\"/var\", {recursive:true}, ()=>{})"' \
                                                       'node fs.rm async'
expect_deny 'node -e "fs.writeFileSync(\"/etc/passwd\", \"\")"' \
                                                       'node fs.writeFileSync truncate'

# =============================================================================
section "Interpreters: Ruby"
# =============================================================================
expect_deny 'ruby -e "FileUtils.rm_rf(\"/var\")"'      'ruby FileUtils.rm_rf'
expect_deny 'ruby -e "FileUtils.rm(\"/etc/passwd\")"'  'ruby FileUtils.rm'
expect_deny 'ruby -e "File.delete(\"/etc/passwd\")"'   'ruby File.delete'
expect_deny 'ruby -e "Dir.rmdir(\"/var\")"'             'ruby Dir.rmdir'
expect_deny 'ruby -e "system(\"rm -rf /\")"'            'ruby system exec'

# =============================================================================
section "Interpreters: Perl"
# =============================================================================
expect_deny 'perl -e "use File::Path; remove_tree(\"/var\")"' \
                                                       'perl remove_tree'
expect_deny 'perl -e "unlink(\"/etc/passwd\")"'         'perl unlink'
expect_deny 'perl -e "rmdir(\"/var\")"'                  'perl rmdir'
expect_deny 'perl -e "system(\"rm -rf /\")"'             'perl system exec'
