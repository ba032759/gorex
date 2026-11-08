# Questions for the owner

Grouped by item. Each says what this pass could not find in Rex's
documentation or the repository.

## B-1 Rex's way of working, on Linux

1. Why GoRex, and not Rex's own Linux server with another terminal, or
   Rex for Mac connected to it? What does the user lack on Linux today?
2. Who is the user: the owner alone, or others who use Rex on a Mac and
   Linux elsewhere?
3. How much of B-1 is Windows? The README treats it as a first-class
   system; is it?

## B-5 What works with Rex works with GoRex

4. Why compatibility with Rex's CLI: agents trained on Rex's
   documentation, scripts moved from a Mac, the owner's own habits, or a
   later switch back to Rex?
5. How far does it go? Exit statuses and JSON shapes, or only command
   names and flags? Finding 17 (flags taken and ignored) and Finding 18
   (flags refused) depend on the answer.

## B-9 Built and kept by agents for one owner

6. Is this a constraint of GoRex, or of the spec experiment only? Should
   it stay a business item?

## D-4 No Lua

7. Why no Lua: size, effort, or a choice against configuration in code?
   Is it rejected for good, or deferred?

## D-5 A local server only, D-7 A protocol of GoRex's own

8. Is remote access (`-S`, hosts, Tailscale) rejected or deferred? The
   next planned feature, access from an Android phone, needs a remote
   client.
9. Was Rex's server API left out on purpose, or for lack of its
   documentation? Should Rex's `rex` be able to drive GoRex's server?

## F-106 Which server, F-120 What rex reads

10. Should `rex` follow `REX_SERVER`, as Rex's does, rather than
    `GOREX_DIR` (Finding 11)?
11. The README says `~/.config/gorex`; the code uses `GoRex`. Which is
    meant (Finding 14)?

## F-109 Where rex runs

12. Is `GOREX_SESSION`, a block's ID, still needed (Finding 12)?

## F-112 and F-142 The default block

13. Should `rex focus left` start from the context block, as every other
    command does, or from the focused block (Finding 2)?

## F-131, F-134, F-135, F-150 Directories of new blocks

14. Which directory should a new block start in without `--cwd`: the
    directory `rex` runs in, or that of the block it goes beside
    (Finding 3)?

## F-135, F-145 Ratios

15. Is `--ratio` from 1 to 99 everywhere (Finding 1)?

## F-101 Timeouts

16. Should `--timeout` limit every request and the connection, as Rex's
    reference says (Finding 4)?

## F-104 Exit statuses

17. With no app connected, should `rex do` and `rex config reload` agree
    (Finding 5)? Which status?
18. When the server goes away, 4 or 1 (Finding 6)?
19. Should a command with several targets resolve them all before acting
    on any (Finding 10)?

## F-152, T-108 JSON keys

20. May `rex block ls --json` change `exitCode` and `lastOutput` to snake
    case, which breaks scripts that read them (Finding 8)?

## F-119 The rex of a block's PATH

21. On Windows, how should a block find the `rex` of its server
    (Finding 9)?

## F-164, F-165 Capture

22. Is 8 MiB of scrollback chosen, or a placeholder for Rex's 64 MiB?
23. Should `capture` keep wrapped lines apart unless `--unwrap`, as Rex
    does?

## F-172 rex status

24. Is `rex status` meant to stay GoRex's own command (Finding 20)?

## F-185, F-186 Agents

25. Should `rex skill setup` install Codex's hook, as Rex does?

## F-122 GoRex's first commands

26. Do scripts still use `list`, `new-tab`, `send-keys` and the old
    `split` flags, or can they go?
