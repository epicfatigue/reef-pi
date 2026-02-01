// front-end/src/utils/timestamp.js

const monthIndex = {
  Jan: 0,
  Feb: 1,
  Mar: 2,
  Apr: 3,
  May: 4,
  Jun: 5,
  Jul: 6,
  Aug: 7,
  Sep: 8,
  Oct: 9,
  Nov: 10,
  Dec: 11
}

export const ParseTimestamp = (v) => {
  // Expected: "Jan-31-16:37, 2026"
  if (!v || v.length < 18) return new Date(NaN)

  const mon = v.slice(0, 3)
  const day = parseInt(v.slice(4, 6), 10)
  const hour = parseInt(v.slice(7, 9), 10)
  const min = parseInt(v.slice(10, 12), 10)
  const year = parseInt(v.slice(14, 18), 10)

  const mi = monthIndex[mon]
  if (mi === undefined) return new Date(NaN)

  // This constructs a LOCAL time Date (matches how reef-pi displays times).
  return new Date(year, mi, day, hour, min, 0, 0)
}
