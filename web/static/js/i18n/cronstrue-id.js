(function () {
  var id = {
    atX0SecondsPastTheMinuteGt20: function () {
      return null;
    },
    atX0MinutesPastTheHourGt20: function () {
      return null;
    },
    commaMonthX0ThroughMonthX1: function () {
      return null;
    },
    commaYearX0ThroughYearX1: function () {
      return null;
    },
    use24HourTimeFormatByDefault: function () {
      return !0;
    },
    anErrorOccuredWhenGeneratingTheExpressionD: function () {
      return "Terjadi kesalahan saat membuat deskripsi ekspresi. Periksa sintaks ekspresi cron.";
    },
    everyMinute: function () {
      return "setiap menit";
    },
    everyHour: function () {
      return "setiap jam";
    },
    atSpace: function () {
      return "Pada ";
    },
    everyMinuteBetweenX0AndX1: function () {
      return "Setiap menit diantara %s dan %s";
    },
    at: function () {
      return "Pada";
    },
    spaceAnd: function () {
      return " dan";
    },
    everySecond: function () {
      return "setiap detik";
    },
    everyX0Seconds: function () {
      return "setiap %s detik";
    },
    secondsX0ThroughX1PastTheMinute: function () {
      return "detik ke %s sampai %s melewati menit";
    },
    atX0SecondsPastTheMinute: function () {
      return "pada %s detik lewat satu menit";
    },
    everyX0Minutes: function () {
      return "setiap %s menit";
    },
    minutesX0ThroughX1PastTheHour: function () {
      return "menit ke %s sampai %s melewati jam";
    },
    atX0MinutesPastTheHour: function () {
      return "pada %s menit melewati jam";
    },
    everyX0Hours: function () {
      return "setiap %s jam";
    },
    betweenX0AndX1: function () {
      return "diantara %s dan %s";
    },
    atX0: function () {
      return "pada %s";
    },
    commaEveryDay: function () {
      return ", setiap hari";
    },
    commaEveryX0DaysOfTheWeek: function () {
      return ", setiap hari %s  dalam seminggu";
    },
    commaX0ThroughX1: function () {
      return ", %s sampai %s";
    },
    commaAndX0ThroughX1: function () {
      return ", dan %s sampai %s";
    },
    first: function () {
      return "pertama";
    },
    second: function () {
      return "kedua";
    },
    third: function () {
      return "ketiga";
    },
    fourth: function () {
      return "keempat";
    },
    fifth: function () {
      return "kelima";
    },
    commaOnThe: function () {
      return ", di ";
    },
    spaceX0OfTheMonth: function () {
      return " %s pada bulan";
    },
    lastDay: function () {
      return "hari terakhir";
    },
    commaOnTheLastX0OfTheMonth: function () {
      return ", pada %s terakhir bulan ini";
    },
    commaOnlyOnX0: function () {
      return ", hanya pada %s";
    },
    commaAndOnX0: function () {
      return ", dan pada %s";
    },
    commaEveryX0Months: function () {
      return ", setiap bulan %s ";
    },
    commaOnlyInX0: function () {
      return ", hanya pada %s";
    },
    commaOnTheLastDayOfTheMonth: function () {
      return ", pada hari terakhir bulan ini";
    },
    commaOnTheLastWeekdayOfTheMonth: function () {
      return ", pada hari kerja terakhir setiap bulan";
    },
    commaDaysBeforeTheLastDayOfTheMonth: function () {
      return ", %s hari sebelum hari terakhir setiap bulan";
    },
    firstWeekday: function () {
      return "hari kerja pertama";
    },
    weekdayNearestDayX0: function () {
      return "hari kerja terdekat %s";
    },
    commaOnTheX0OfTheMonth: function () {
      return ", pada %s bulan ini";
    },
    commaEveryX0Days: function () {
      return ", setiap %s hari";
    },
    commaBetweenDayX0AndX1OfTheMonth: function () {
      return ", antara hari %s dan %s dalam sebulan";
    },
    commaOnDayX0OfTheMonth: function () {
      return ", pada hari %s dalam sebulan";
    },
    commaEveryHour: function () {
      return ", setiap jam";
    },
    commaEveryX0Years: function () {
      return ", setiap %s tahun";
    },
    commaStartingX0: function () {
      return ", mulai pada %s";
    },
    daysOfTheWeek: function () {
      return ["Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"];
    },
    monthsOfTheYear: function () {
      return [
        "Januari",
        "Februari",
        "Maret",
        "April",
        "Mei",
        "Juni",
        "Juli",
        "Agustus",
        "September",
        "Oktober",
        "November",
        "Desember",
      ];
    },
    onTheHour: function () {
      return "tepat pada jam";
    },
  };
  if (cronstrue.default && cronstrue.default.locales) {
    cronstrue.default.locales["id"] = id;
  } else if (cronstrue.locales) {
    cronstrue.locales["id"] = id;
  }
})();
