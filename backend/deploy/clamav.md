# Pemeriksaan lampiran pengumuman

Siapkan ClamAV sebelum merilis perubahan lampiran. Tanpa pemindai, API tetap berjalan tetapi unggahan dan pembacaan lampiran lama ditolak dengan `FILE_SCAN_UNAVAILABLE`. Tidak ada sakelar untuk melewati pemeriksaan, termasuk di development.

1. Pasang paket `clamav`, `clamav-daemon`, dan `clamav-freshclam` dari repositori OS server yang didukung. Perbarui paket dan signature secara rutin. Jalankan freshclam sebagai service; pastikan pembaruan berhasil dan clamd memuat signature terbaru. Jangan menjalankan freshclam manual bersamaan dengan servicenya.
2. Tambahkan pengaturan berikut ke konfigurasi clamd yang aktif (lokasi mengikuti paket OS). Pilih socket Unix dengan izin akses untuk user API, atau TCP **loopback saja**. Contoh TCP:

   ```conf
   TCPSocket 3310
   TCPAddr 127.0.0.1
   ScanPDF yes
   ScanArchive yes
   HeuristicAlerts yes
   AlertEncrypted yes
   AlertExceedsMax yes
   AlertBrokenMedia yes
   StreamMaxLength 6M
   MaxFileSize 6M
   MaxScanSize 100M
   MaxScanTime 10000
   MaxRecursion 16
   MaxFiles 1000
   ```

   Jangan publikasikan port clamd ke internet. Protokol ini tidak menyediakan autentikasi atau TLS. Batas dan dokumen terenkripsi harus menghasilkan penolakan, bukan hasil bersih. Pastikan RAM cukup untuk daemon dan basis data signature sebelum restart.
3. Set environment API: `CLAMAV_ADDRESS=127.0.0.1:3310` atau path absolut socket, misalnya `/run/clamav/clamd.ctl`. Pertahankan `SELFIE_DIR` dan kunci enkripsi yang sudah dipakai. Restart clamd setelah perubahan konfigurasi dan API setelah perubahan environment.
4. Verifikasi di staging dengan JPEG, PNG transparan, PDF beberapa halaman, dan berkas uji EICAR resmi. EICAR harus ditolak; saat clamd dihentikan, unggahan harus menghasilkan 503 tanpa metadata/berkas baru. Mulai lagi clamd setelah pengujian. Uji sasaran divisi lain tetap 404.
5. Buka pengumuman di pop-up dan detail Android serta pratinjau web. Periksa isi PDF/gambar muncul otomatis, halaman PDF dapat digulir di Android/dipindah di web, dan gambar dapat diperbesar. Web memakai PDF.js dari aset situs sendiri untuk merender piksel tanpa skrip PDF. PDF yang belum bisa dirender tetap bisa diunduh.

Kompresi gzip dilakukan sebelum enkripsi AES-GCM dan dibuka kembali oleh API. Metadata `size_bytes` adalah ukuran berkas yang dikirim ke perangkat setelah optimasi gambar. Berkas yang sudah padat tidak selalu dapat diperkecil; gzip hanya disimpan jika lebih kecil. PDF tidak ditulis ulang sehingga tanda tangan digital dan kualitasnya tetap dipertahankan. Gambar dikodekan ulang, metadata dibuang, orientasi EXIF JPEG diterapkan, dan sisi terpanjang dibatasi 2560 px.

Berkas baru memakai akhiran storage internal `.safe-v1` atau `.safe-v1.gz`. Ini menandai versi proses, bukan ekstensi yang diberikan ke pengguna. Lampiran lama tetap kompatibel dan dipindai saat dibaca setelah pemeriksaan hak akses; kompresi baru berlaku untuk unggahan baru. Belum ada pemrosesan massal terhadap lampiran lama. Jangan mengganti nama kunci storage secara manual. Backup tetap mencakup database, berkas terenkripsi, dan kunci enkripsinya.

Pemindaian mengurangi risiko, tidak menjamin semua malware baru terdeteksi. Kesehatan freshclam dan usia signature perlu dipantau oleh operator. Tes unit protokol memakai daemon tiruan; pengujian EICAR dengan daemon asli diperlukan saat deployment.

Referensi: [protokol clamd](https://docs.clamav.net/manual/Usage/ClamdProtocol.html), [opsi resmi clamd](https://github.com/Cisco-Talos/clamav/blob/main/docs/man/clamd.conf.5.in).
