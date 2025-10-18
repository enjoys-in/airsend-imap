import Imap from 'node-imap'

var imap = new Imap({
  user: "mullayam06@airsend.in",
  password: "4de75c41c9e04bd2",
  // user: "user1@example.com",
  // password: "pass",
  host: 'localhost',
  port: 143,
  tls: false,
  debug(info) {
    console.log(info)
  },

});
imap.once('ready', function () {
  imap.getBoxes(function (err, boxes) {
    if (err) throw err;
    
      imap.openBox('INBOX', false, function (err, box) {
    if (err) throw err;
    console.log("Opened box:", box);
    // imap.end();
  });
  });

});

imap.once('error', function (err) {
  console.log(err);
});

imap.once('end', function () {
  console.log('Connection ended');
});

imap.connect();