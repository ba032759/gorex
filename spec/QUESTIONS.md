# Questions for the owner

Grouped by item. Each says what this pass could not find in Rex's
documentation or the repository.

## B-1 Rex's way of working, on Linux

1. Why GoRex, and not Rex's own Linux server with another terminal, or
   Rex for Mac connected to it? What does the user lack on Linux today?
   *Answered (owner, 2026-11-09):* GoRex keeps its own server for now;
   examine this again when Rex's server is stable (D-11).
2. Who is the user: the owner alone, or others who use Rex on a Mac and
   Linux elsewhere?
   *Answered (2026-11-09):* the owner alone, on one Linux laptop.
3. How much of B-1 is Windows? The README treats it as a first-class
   system; is it?

   *Answered (2026-11-09):* none: GoRex is Linux only (B-7, D-16).
## B-5 What works with Rex works with GoRex

4. Why compatibility with Rex's CLI: agents trained on Rex's
   documentation, scripts moved from a Mac, the owner's own habits, or a
   later switch back to Rex?
   *Answered (2026-11-09):* unknown; B-5 keeps `**Why.** Unknown.` The owner accepts B-5 as it is.
5. How far does it go? Exit statuses and JSON shapes, or only command
   names and flags? Finding 17 (flags taken and ignored) and Finding 18
   (flags refused) depend on the answer.

   *Answered (2026-11-09):* as far as Rex's reference goes: commands, flags, exit statuses and JSON keys (Findings 8, 18).
## B-9 Built and kept by agents for one owner

6. Is this a constraint of GoRex, or of the spec experiment only? Should
   it stay a business item?
   *Answered (2026-11-09):* background, not a need; B-9 is rejected and
   its text is at the top of `business/needs.md`.

## D-4 No Lua

7. Why no Lua: size, effort, or a choice against configuration in code?
   Is it rejected for good, or deferred?

   *Answered (2026-11-09):* unknown. D-4 is accepted as it is: no Lua.
## D-5 A local server only, D-7 A protocol of GoRex's own

8. Is remote access (`-S`, hosts, Tailscale) rejected or deferred? The
   next planned feature, access from an Android phone, needs a remote
   client.
   *Answered (2026-11-09):* deferred: the phone feature will decide it, through `spec-propose`. D-5 holds until then.
9. Was Rex's server API left out on purpose, or for lack of its
   documentation? Should Rex's `rex` be able to drive GoRex's server?

   *Answered (2026-11-09):* answered by D-11 and D-7: GoRex keeps its own server and protocol.
## F-106 Which server, F-120 What rex reads

10. Should `rex` follow `REX_SERVER`, as Rex's does, rather than
    `GOREX_DIR` (Finding 11)?
   *Answered (2026-11-09):* yes: `rex` reads `REX_SERVER` (`unix:PATH`) before `GOREX_DIR` (Finding 11). `-S` stays refused (D-5).
11. The README says `~/.config/gorex`; the code uses `GoRex`. Which is
    meant (Finding 14)?

   *Answered (2026-11-09):* `GoRex`, as the code has it; the README is corrected (Finding 14).
## F-109 Where rex runs

12. Is `GOREX_SESSION`, a block's ID, still needed (Finding 12)?

   *Answered (2026-11-09):* no: `GOREX_SESSION` goes (Finding 12).
## F-112 and F-142 The default block

13. Should `rex focus left` start from the context block, as every other
    command does, or from the focused block (Finding 2)?

   *Answered (2026-11-09):* from the context block, as every other command (F-112, Finding 2).
## F-132 Directories of new blocks

14. Which directory should a new block start in without `--cwd`: the
    directory `rex` runs in, or that of the block it goes beside
    (Finding 3)?

   *Answered (2026-11-09):* beside a block: that block's directory; otherwise: the directory `rex` runs in. One rule for every command (Finding 3).
## F-135, F-145 Ratios

15. Is `--ratio` from 1 to 99 everywhere (Finding 1)?

   *Answered (2026-11-09):* yes (D-14).
## F-101 Timeouts

16. Should `--timeout` limit every request and the connection, as Rex's
    reference says (Finding 4)?

   *Answered (2026-11-09):* yes, as Rex's reference says: every connection and every request that does not stream (Finding 4).
## F-104 Exit statuses

17. With no app connected, should `rex do` and `rex config reload` agree
    (Finding 5)? Which status?
   *Answered (2026-11-09):* yes: both exit with 1, as Rex's table says for a command that failed (Finding 5).
18. When the server goes away, 4 or 1 (Finding 6)?
   *Answered (2026-11-09):* 4, "the server could not be reached", for every command (Finding 6).
19. Should a command with several targets resolve them all before acting
    on any (Finding 10)?

   *Answered (2026-11-09):* yes, as Rex's reference says of `rex swap`: resolve every target, then act; IF one fails, nothing changes (Finding 10).
## F-130, T-108 JSON keys

20. May `rex block ls --json` change `exitCode` and `lastOutput` to snake
    case, which breaks scripts that read them (Finding 8)?

   *Answered (2026-11-09):* yes: snake case, as Rex's keys (Finding 8).
## F-119 The rex of a block's PATH

21. On Windows, how should a block find the `rex` of its server
    (Finding 9)?

   *Answered (2026-11-09):* none: GoRex is Linux only (D-16).
## F-164, F-165 Capture

22. Is 8 MiB of scrollback chosen, or a placeholder for Rex's 64 MiB?
   *Answered (2026-11-09):* about 64 MiB, as Rex's reference says.
23. Should `capture` keep wrapped lines apart unless `--unwrap`, as Rex
    does?

   *Answered (2026-11-09):* yes, as Rex: wrapped lines stay apart unless `--unwrap`.
## F-172 rex status

24. Is `rex status` meant to stay GoRex's own command (Finding 20)?

   *Answered (2026-11-09):* yes (Finding 20).
## F-185, F-186 Agents

25. Should `rex skill setup` install Codex's hook, as Rex does?

   *Answered (2026-11-09):* deferred, until it is checked against Codex's own documentation.
## F-122 GoRex's first commands

26. Do scripts still use `list`, `new-tab`, `send-keys` and the old
    `split` flags, or can they go?

   *Answered (2026-11-09):* they stay (D-3).
## D-12 A native app, without a web view

27. Why native and without a web view: size, speed, feel, or the wish to
    try MyGo? Is a web view ruled out for a later client, such as a
    phone?

## F-142, F-143 Focus in a zoomed window

28. Should `rex focus next` in a zoomed window end the zoom, as `rex
    focus BLOCK` does, or not move, as `rex focus left` does
    (Finding 28)?

   *Answered (2026-11-09):* it ends the zoom, as `rex focus BLOCK` does: focusing another block ends a window's zoom, by any command (Finding 28).
## F-193 Inspect a client

29. Should `rex client inspect` take `--json` and `-C`, and match a
    client as every other target does (Findings 7, 29)?
    *Answered (2026-11-09):* yes, as Rex's reference says (F-193).
## F-174 rex do

30. Should `rex do -b BLOCK` act on `BLOCK`, as Rex's does, or should
    `-s` and `-b` be ignored and say so, as Finding 17's flags (Finding
    36)?

    *Answered (2026-11-09):* yes, as Rex's (Finding 36).
## F-178, F-179 The keymap's errors

31. Should `rex keymap` exit with 1 when the keymap has an error, as
    `rex config check` and Rex's `rex keymap` do (Finding 37)?

    *Answered (2026-11-09):* yes, as Rex's (Finding 37).
## F-430 The server's log

32. Should `rex server logs` print the newest 200 lines by default and
    take `--all`, as Rex does (Finding 38)?

    *Answered (2026-11-09):* yes, as Rex's, with `--all` (Finding 38).