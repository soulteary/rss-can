const config = {
    Name: "Yahoo",
    URL: "https://us.yahoo.com/",
    Mode: "csr",
    IdByRegexp: "\\/(\\.+)\.html$",
    Proxy: "http://10.11.12.90:11080",
  
    Title: "h3",
    Author: "Yahoo",
    Category: ".Fw\(b\).Tt\(c\)",
    Link: "h3 a",
  };
  potted.SetConfig(config).SetScope("#main .stream-items");
  