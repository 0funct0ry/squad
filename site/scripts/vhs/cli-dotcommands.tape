# Run from the repo root:
#   cp examples/ecommerce.db /tmp/cli-demo-dot.db
#   make build
#   VHS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
#     vhs site/scripts/vhs/cli-dotcommands.tape
Output "site/public/casts/cli-dotcommands.gif"

Require bash

Set Shell "bash"
Set FontSize 18
Set Width 1100
Set Height 680
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 45ms

Type "./bin/squad cli --write /tmp/cli-demo-dot.db"
Enter
Sleep 1.5s

Type ".timer on"
Enter
Sleep 500ms

Type "select count(*) from orders;"
Enter
Sleep 1.5s

Type ".explain select * from orders where customer_id = 7;"
Enter
Sleep 2.5s

Type "select * from products where price > 100 limit 20;"
Enter
Sleep 1.5s

Type ".grep -r Laptop"
Enter
Sleep 2.5s

Type ".constraints orders"
Enter
Sleep 2.5s

Type ".quit"
Enter
Sleep 1s
