function gwt --description 'Git worktree wrapper that changes directory on add'
    set git_wt_binary "$HOME/.local/bin/git-wt-go"

    # Check if this is an 'add' or 'sw' command
    if test "$argv[1]" = "add" -o "$argv[1]" = "sw"
        if test (count $argv) -lt 2
            echo "Error: $argv[1] requires a worktree name"
            $git_wt_binary
            return 1
        end
        
        # Run git-wt add/sw normally
        set new_dir ($git_wt_binary $argv)
        set exit_code $status
        
        if test $exit_code -eq 0 -a -n "$new_dir"
            echo "Changing directory to workspace $new_dir"
            cd $new_dir
        end

        return $exit_code
    else
        # For all other commands, just pass through
        $git_wt_binary $argv
    end
end
