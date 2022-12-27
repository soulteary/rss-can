/**
 * convert a datetime string to a unix timestamp
 * @param {string} input
 * @returns {string | number} "" or "unix number"
 */
function ConvertAgoToUnix(input) {
  var s = (input || "").trim().toLowerCase();
  if (!s) return "";
  var t = moment();
  if (s.indexOf("ago") > -1 || s.indexOf("前") > -1) {
    if (s.indexOf("second") > -1 || s.indexOf("sec") > -1 || s.indexOf("秒") > -1) {
      s = s.replace(/\D/g, "");
      return t.subtract(parseInt(s, 10), "seconds").unix();
    }
    if (s.indexOf("minute") > -1 || s.indexOf("min") > -1 || s.indexOf("分") > -1) {
      s = s.replace(/\D/g, "");
      return t.subtract(parseInt(s, 10), "minutes").unix();
    }
    if (s.indexOf("hour") > -1 || s.indexOf("小时") > -1) {
      s = s.replace(/\D/g, "");
      return t.subtract(parseInt(s, 10), "hours").unix();
    }
    if (s.indexOf("day") > -1 || s.indexOf("天") > -1) {
      s = s.replace(/\D/g, "");
      return t.subtract(parseInt(s, 10), "days").unix();
    }
    if (s.indexOf("week") > -1 || s.indexOf("周") > -1) {
      s = s.replace(/\D/g, "");
      return t.subtract(parseInt(s, 10), "week").unix();
    }
    if (s.indexOf("month") > -1 || s.indexOf("月") > -1) {
      s = s.replace(/\D/g, "");
      return t.subtract(parseInt(s, 10), "month").unix();
    }
    if (s.indexOf("year") > -1 || s.indexOf("年") > -1) {
      s = s.replace(/\D/g, "");
      return t.subtract(parseInt(s, 10), "year").unix();
    }
    return s;
  }
  return s;
}
