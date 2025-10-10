import Imap from 'node-imap'

var imap = new Imap({
    user: 'mygmailname',
    password: 'mygmailpassword',
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

imap.once('error', function(err) {
  console.log(err);
});
 
imap.once('end', function() {
  console.log('Connection ended');
});
 
imap.connect();