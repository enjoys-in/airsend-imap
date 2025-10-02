Core Features Every IMAP Server Needs

Basic Commands

LOGIN / AUTHENTICATE → user authentication

LOGOUT → clean session close

CAPABILITY → report server features (important, e.g., IMAP4rev1, STARTTLS)

NOOP → keepalive, prevents client timeout

Mailbox / Folder Management

LIST → list all mailboxes/folders

LSUB → list subscribed folders

SELECT → open a mailbox for reading

EXAMINE → read-only select

Message Fetching

FETCH → retrieve message headers, body, flags

Examples:

BODY[] → full message

BODY.PEEK[] → same, without setting \Seen

FLAGS → message flags

UID → unique IDs for messages

SEARCH → search messages (optional for minimal server, but Thunderbird expects it)

Message Appending

APPEND → client can upload messages (important for Outlook, Gmail apps, and mobile devices syncing sent messages)

Message Deletion / Expunge

STORE +FLAGS \Deleted

EXPUNGE → actually remove messages

Clients like Outlook and Thunderbird rely on these to mark trash / delete messages

Flags

Standard flags:

\Seen → read/unread

\Answered → replied

\Flagged → important/starred

\Deleted → marked for deletion

\Draft → draft

Optional: \Recent, \Junk

UIDs

UID FETCH, UID STORE, UID SEARCH → unique identifiers per message

Thunderbird and Outlook use UIDs to avoid re-downloading messages

IMAP Extensions (Optional but Recommended)

UIDPLUS → better message append/fetch handling

IDLE → push notifications for new mail (Thunderbird uses IDLE)

SORT / CONDSTORE / QRESYNC → improves sync performance

LITERAL+ → handle large messages efficiently

Security

STARTTLS or direct IMAPS (port 993) → clients reject plaintext IMAP on the internet

Authentication mechanisms like PLAIN, LOGIN, CRAM-MD5, XOAUTH2 (for Gmail-like OAuth)

Optional Enhancements

XLIST → folder type hints for Gmail (INBOX, Sent, Trash, Spam)

METADATA → store per-folder metadata

ID → client can query server software/version

🔹 Minimal Viable IMAP for Clients

If you want Thunderbird / Outlook / Gmail App to “just work”, at minimum implement:

CAPABILITY

LOGIN / AUTHENTICATE

LIST / LSUB

SELECT / EXAMINE

FETCH BODY[], FLAGS

UID FETCH

APPEND (for sent items)

STORE +FLAGS \Seen / \Deleted

EXPUNGE

STARTTLS or IMAPS

Everything else (SEARCH, IDLE, CONDSTORE) is optional but improves UX.

🔹 Thunderbird / Outlook Gotchas

Thunderbird

Expects UID FETCH, FLAGS for proper sync

Uses IDLE for push notifications

Uses \Recent to highlight new messages

Outlook

Very picky about APPEND behavior (Sent items)

Needs standard flags to work correctly

Some versions require UIDPLUS for reliable sync

Gmail / iOS / Android

Mostly standard IMAP

Uses IDLE or periodic polling

Expects proper folder attributes (INBOX, Sent, Trash)

💡 Summary:
To support all major clients, your Go IMAP server should implement:

Authentication & CAPABILITY

Mailbox listing & selection

Message FETCH with BODY[] & FLAGS

UID tracking

STORE / APPEND / EXPUNGE

STARTTLS or IMAPS

Optional but highly recommended:

IDLE, UIDPLUS, CONDSTORE, SEARCH, folder attributes (XLIST)