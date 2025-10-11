import Imap from 'node-imap'

var imap = new Imap({
  user: "mullayam06@airsend.in",
  password: "4de75c41c9e04bd2",
  host: 'localhost',
  port: 143,
  tls: false,
  debug(info) {
    console.log(info)
  },

});
imap.once('ready', function () {
  imap.openBox('INBOX', false, function (err, box) {
    if (err) throw err;
    var f = box.messages.total;
    console.log(f + ' messages');
    imap.end();
  });
});

imap.once('error', function (err) {
  console.log(err);
});

imap.once('end', function () {
  console.log('Connection ended');
});

imap.connect();