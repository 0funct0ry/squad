# Run from the repo root:
#   cp examples/ecommerce.db /tmp/cli-demo-tpl.db
#   make build
#   VHS_CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
#     vhs site/scripts/vhs/cli-templates.tape
Output "site/public/casts/cli-templates.gif"

Require bash

Set Shell "bash"
Set FontSize 18
Set Width 1100
Set Height 680
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 45ms

Type "./bin/squad cli --write /tmp/cli-demo-tpl.db"
Enter
Sleep 1.5s

Type ".echo {{firstName}} {{lastName}} <{{email}}>"
Enter
Sleep 2s

Type ".seed customers 5"
Enter
Sleep 2s

Type "select id, name, email from customers order by id desc limit 5;"
Enter
Sleep 2.5s

Type `.repeat 3 "insert into categories (name) values ('{{word}}');"`
Enter
Sleep 2.5s

Type "select * from categories order by id desc limit 3;"
Enter
Sleep 2.5s

Type ".quit"
Enter
Sleep 1s
