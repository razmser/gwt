# Fish completion for gwt command

# Complete subcommands
complete -c gwt -f -n "__fish_use_subcommand" -a "add" -d "Create new worktree and cd into it"
complete -c gwt -f -n "__fish_use_subcommand" -a "switch" -d "Switch to existing worktree"
complete -c gwt -f -n "__fish_use_subcommand" -a "list" -d "List all worktrees"
complete -c gwt -f -n "__fish_use_subcommand" -a "remove" -d "Remove worktree"
complete -c gwt -f -n "__fish_use_subcommand" -a "cleanup" -d "Remove worktree"

# Helper function to get worktree names
function __gwt_worktree_names
    set -l current_branch (git rev-parse --abbrev-ref HEAD 2>/dev/null)
    # Strip wt/ prefix from current branch if present
    if string match -q "wt/*" $current_branch
        set current_branch (string replace "wt/" "" $current_branch)
    end

    # Get all worktrees and filter out the current one
    $HOME/.local/bin/git-wt-go list 2>/dev/null | while read -l wt
        if test "$wt" != "$current_branch"
            echo $wt
        end
    end
end

# Complete worktree names for 'rm' and 'sw' subcommands
complete -c gwt -f -n "__fish_seen_subcommand_from rm" -a "(__gwt_worktree_names)"
complete -c gwt -f -n "__fish_seen_subcommand_from remove" -a "(__gwt_worktree_names)"
complete -c gwt -f -n "__fish_seen_subcommand_from sw" -a "(__gwt_worktree_names)"
complete -c gwt -f -n "__fish_seen_subcommand_from switch" -a "(__gwt_worktree_names)"

