| Command                     | Purpose                                                |
| --------------------------- | ------------------------------------------------------ |
| `CAPABILITY`                | List supported extensions (IDLE, UIDPLUS, etc.)        |
| `NOOP`                      | Keep connection alive, return pending updates          |
| `LOGOUT`                    | End session                                            |
| `STARTTLS`                  | Upgrade to TLS (if not implicit 993)                   |
| `LOGIN` / `AUTHENTICATE`    | User login (plain password or SASL mechanism)          |
| `SELECT` / `EXAMINE`        | Open a mailbox (read-write / read-only)                |
| `CREATE`                    | Create new mailbox                                     |
| `DELETE`                    | Delete mailbox                                         |
| `RENAME`                    | Rename mailbox                                         |
| `SUBSCRIBE` / `UNSUBSCRIBE` | Manage subscribed folders (clients show in UI)         |
| `LIST` / `LSUB`             | List folders                                           |
| `STATUS`                    | Return stats (message count, unseen, UIDNEXT, etc.)    |
| `APPEND`                    | Add a message to a mailbox                             |
| `CHECK`                     | Sync state                                             |
| `CLOSE`                     | Close mailbox                                          |
| `EXPUNGE`                   | Permanently remove deleted messages                    |
| `SEARCH`                    | Find messages by criteria (FROM, SUBJECT, SINCE, etc.) |
| `FETCH`                     | Get message data (headers, body, flags)                |
| `STORE`                     | Change message flags (Seen, Deleted, etc.)             |
| `COPY`                      | Copy message to another mailbox                        |
| `UID`                       | Version of FETCH/STORE/COPY/SEARCH using unique IDs    |


✅ Authorize
✅ CreateMailbox
✅ GetMessageLiteral
✅ GetMailboxVisibility
✅ UpdateMailboxName
✅ DeleteMailbox
✅ CreateMessage
✅ AddMessagesToMailbox
✅ RemoveMessagesFromMailbox
✅ MoveMessages
✅ MarkMessagesSeen
✅ MarkMessagesFlagged
✅ ListMailboxes
✅ ListMessages
✅ GetMessageFlags
✅ ListMessages
✅ ListMessages







