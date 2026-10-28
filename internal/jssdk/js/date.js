// Date conversion deliberately avoids Moment's implementation-defined native
// Date fallback. Unsupported text and impossible dates are conversion errors.
function ValidUnixDate(value, input) {
  if (!value.isValid()) throw new Error("invalid date: " + input);
  const timestamp = value.unix();
  if (!Number.isSafeInteger(timestamp)) throw new Error("invalid date timestamp: " + input);
  return timestamp;
}

function ConvertAgoToUnix(input) {
  const text = String(input == null ? "" : input).trim();
  const match = /^(\d+)\s*(seconds?|secs?|minutes?|mins?|hours?|days?|weeks?|months?|years?)\s+ago$/i.exec(text)
    || /^(\d+)\s*(秒钟?|分钟?|小时|天|周|个?月|年)\s*前$/.exec(text);
  if (!match) throw new Error("invalid relative date: " + text);
  const count = Number(match[1]);
  if (!Number.isSafeInteger(count)) throw new Error("invalid relative date: " + text);
  const units = {
    sec: "seconds", secs: "seconds", second: "seconds", seconds: "seconds", "秒": "seconds", "秒钟": "seconds",
    min: "minutes", mins: "minutes", minute: "minutes", minutes: "minutes", "分": "minutes", "分钟": "minutes",
    hour: "hours", hours: "hours", "小时": "hours",
    day: "days", days: "days", "天": "days",
    week: "weeks", weeks: "weeks", "周": "weeks",
    month: "months", months: "months", "月": "months", "个月": "months",
    year: "years", years: "years", "年": "years"
  };
  return ValidUnixDate(moment().subtract(count, units[match[2].toLowerCase()]), text);
}

function ConvertStrToUnix(input) {
  const text = String(input == null ? "" : input).trim();
  if (!text) throw new Error("date must not be empty");
  if (/ago$|前$/i.test(text)) return ConvertAgoToUnix(text);

  // Numeric calendar components accept both padded and unpadded Chinese
  // dates. Moment's array constructor validates rather than rolling over invalid
  // dates. A missing year uses the current local calendar year.
  let match = /^(?:(\d{4})年)?(\d{1,2})月(\d{1,2})日(?:\s+(\d{1,2}):(\d{2})(?::(\d{2}))?)?$/.exec(text);
  if (match) {
    const year = match[1] == null ? new Date().getFullYear() : Number(match[1]);
    return ValidUnixDate(moment([year, Number(match[2]) - 1, Number(match[3]), Number(match[4] || 0), Number(match[5] || 0), Number(match[6] || 0)]), text);
  }
  match = /^(\d{4})年(?:(\d{1,2})月)?$/.exec(text);
  if (match) return ValidUnixDate(moment([Number(match[1]), Number(match[2] || 1) - 1, 1]), text);

  match = /^(\d{4})\/(\d{1,2})\/(\d{1,2})(?:\s+(\d{1,2}):(\d{2})(?::(\d{2}))?)?$/.exec(text);
  if (match) return ValidUnixDate(moment([Number(match[1]), Number(match[2]) - 1, Number(match[3]), Number(match[4] || 0), Number(match[5] || 0), Number(match[6] || 0)]), text);

  return ValidUnixDate(moment(text, [moment.ISO_8601, moment.RFC_2822], true), text);
}
