import type { Metadata } from "next";
import Link from "next/link";
import { Logo } from "@/components/logo";

export const metadata: Metadata = {
  title: "Kebijakan Privasi",
  description: "Kebijakan privasi aplikasi dan web HRIS Al Bahjah untuk Pejuang Lembaga Pengembangan Dakwah Al-Bahjah.",
};

/** Tanggal berlaku. Ubah bersama isi kebijakan dan umumkan perubahannya kepada Pejuang. */
const BERLAKU = "17 November 2026";

type Bagian = { id: string; judul: string; isi: React.ReactNode };

function Daftar({ items }: { items: React.ReactNode[] }) {
  return (
    <ul className="mt-2 list-disc space-y-1.5 pl-5">
      {items.map((item, i) => (
        <li key={i}>{item}</li>
      ))}
    </ul>
  );
}

function Tabel({ kepala, baris }: { kepala: string[]; baris: React.ReactNode[][] }) {
  return (
    <div className="mt-3 overflow-x-auto rounded-lg border border-slate-200">
      <table className={`w-full text-left text-sm ${kepala.length > 2 ? "min-w-[560px]" : ""}`}>
        <thead className="bg-slate-50 text-slate-700">
          <tr>
            {kepala.map((k) => (
              <th key={k} className="px-3 py-2 font-semibold">
                {k}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200">
          {baris.map((b, i) => (
            <tr key={i} className="align-top">
              {b.map((sel, j) => (
                <td key={j} className="px-3 py-2">
                  {sel}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

const bagian: Bagian[] = [
  {
    id: "pendahuluan",
    judul: "Pendahuluan",
    isi: (
      <>
        <p>
          Kebijakan Privasi ini menjelaskan bagaimana Lembaga Pengembangan Dakwah Al-Bahjah (&quot;Lembaga&quot;, &quot;kami&quot;) mengumpulkan,
          menggunakan, menyimpan, melindungi, dan menghapus data pribadi melalui sistem <strong>HRIS Al Bahjah</strong>, yang terdiri atas
          aplikasi Android HRIS Al Bahjah dan web admin SDM.
        </p>
        <p className="mt-3">
          Kebijakan ini disusun dengan mengacu pada Undang-Undang Nomor 27 Tahun 2022 tentang Pelindungan Data Pribadi (UU PDP) dan
          berlaku bagi seluruh pengguna sistem, yaitu Pejuang (pegawai), atasan (kepala subdivisi dan kepala divisi), staf SDM, dan
          administrator sistem.
        </p>
        <p className="mt-3">
          HRIS Al Bahjah adalah sistem internal kepegawaian. Akun hanya dibuat oleh bagian SDM untuk Pejuang Lembaga dan tidak tersedia
          untuk pendaftaran umum.
        </p>
      </>
    ),
  },
  {
    id: "pengendali",
    judul: "Pengendali Data",
    isi: (
      <>
        <p>
          Pengendali data pribadi dalam sistem ini adalah <strong>Lembaga Pengembangan Dakwah Al-Bahjah</strong>, Cirebon, Jawa Barat.
          Sistem dikembangkan dan dikelola oleh <strong>PUTI (Pusat Teknologi Informasi) Al Bahjah</strong>, sedangkan bagian{" "}
          <strong>SDM</strong> mengelola data kepegawaian sehari-hari.
        </p>
        <p className="mt-3">
          Pertanyaan, permintaan, atau keluhan terkait data pribadi dapat disampaikan melalui saluran pada bagian{" "}
          <a href="#kontak" className="font-medium text-primary-700 underline">
            Kontak
          </a>
          .
        </p>
      </>
    ),
  },
  {
    id: "data-dikumpulkan",
    judul: "Data yang Kami Kumpulkan",
    isi: (
      <>
        <p>Kami hanya mengumpulkan data yang diperlukan untuk menjalankan fungsi kepegawaian. Rinciannya sebagai berikut.</p>

        <h3 className="mt-4 font-semibold text-slate-900">a. Data identitas dan kepegawaian</h3>
        <Daftar
          items={[
            "Nama lengkap, NIPY (Nomor Induk Pegawai Yayasan), dan jenis kelamin.",
            "Cabang, divisi, subdivisi, jabatan, tanggal bergabung, status kepegawaian, serta pengaturan kerja (di kantor, WFA, atau hybrid).",
            "Nomor WhatsApp dan alamat email.",
            "Status pernikahan, keterangan memiliki anak, dan jumlah anak.",
            "Ubar (upah) pokok dan data terkait penghitungan ubar.",
          ]}
        />

        <h3 className="mt-4 font-semibold text-slate-900">b. Data anak Pejuang</h3>
        <p className="mt-1">
          Untuk administrasi kepegawaian dan kesejahteraan keluarga, kami dapat menyimpan data anak Pejuang: nama, jenis kelamin, tempat
          dan tanggal lahir, urutan anak, status (kandung, angkat, atau tiri), jenjang pendidikan, dan nama sekolah. Data ini hanya dapat
          dilihat oleh Pejuang yang bersangkutan, bagian SDM, dan administrator sistem, <strong>tidak</strong> oleh atasan.
        </p>

        <h3 className="mt-4 font-semibold text-slate-900">c. Data akun dan keamanan</h3>
        <Daftar
          items={[
            "Kata sandi, yang hanya disimpan dalam bentuk hash satu arah (argon2id) sehingga tidak dapat dibaca siapa pun, termasuk kami.",
            "Kata sandi awal yang dibuat sistem untuk akun baru. Kata sandi ini berlaku 72 jam dan wajib diganti saat pertama kali masuk.",
            "Pengaturan verifikasi dua langkah (MFA) untuk akun pengelola di web admin.",
            "Data sesi masuk, alamat IP, waktu masuk dan keluar, serta catatan percobaan masuk yang tidak berhasil.",
          ]}
        />

        <h3 className="mt-4 font-semibold text-slate-900">d. Data biometrik (wajah)</h3>
        <Daftar
          items={[
            "Foto selfie yang diambil saat pendaftaran wajah dan saat presensi, termasuk foto gerakan (misalnya kedip, senyum, atau menoleh) untuk memastikan yang difoto adalah orang sungguhan.",
            "Template wajah, yaitu deretan angka hasil olahan foto wajah oleh server. Template ini dipakai untuk mencocokkan wajah Anda dengan data terdaftar dan tidak dapat diubah kembali menjadi foto.",
          ]}
        />
        <p className="mt-2">
          Data biometrik termasuk data pribadi yang bersifat spesifik menurut UU PDP. Data ini hanya diproses setelah Anda memberikan
          persetujuan secara tegas di aplikasi.
        </p>

        <h3 className="mt-4 font-semibold text-slate-900">e. Data lokasi</h3>
        <p className="mt-1">
          Koordinat lokasi dan tingkat akurasinya diambil <strong>hanya pada saat</strong> Anda menekan tombol presensi atau mendaftarkan
          wajah. Aplikasi <strong>tidak</strong> melacak lokasi di latar belakang dan tidak merekam lokasi di luar momen tersebut.
        </p>

        <h3 className="mt-4 font-semibold text-slate-900">f. Data perangkat</h3>
        <Daftar
          items={[
            "ID instalasi acak yang dibuat aplikasi lalu disimpan dalam bentuk hash. Kami tidak mengambil IMEI, nomor seri, atau nomor telepon perangkat.",
            "Model perangkat dan versi Android.",
            "Kunci publik perangkat yang dibuat di penyimpanan aman (Android Keystore), untuk membuktikan bahwa permintaan presensi berasal dari perangkat yang terdaftar.",
            "Hasil pemeriksaan keaslian aplikasi dan perangkat (Google Play Integrity atau Android Key Attestation), serta indikasi root, lokasi palsu (mock location), atau emulator.",
          ]}
        />

        <h3 className="mt-4 font-semibold text-slate-900">g. Data presensi dan pengajuan</h3>
        <Daftar
          items={[
            "Waktu masuk dan pulang, lokasi kerja, status kehadiran (hadir, terlambat, tidak hadir, cuti, dan lainnya), serta catatan peninjauan oleh SDM.",
            "Riwayat upaya presensi, termasuk yang belum berhasil, beserta alasan teknisnya dan skor risiko.",
            "Pengajuan cuti dan izin: jenis, tanggal, alasan, serta riwayat persetujuan.",
            "Status baca pengumuman.",
          ]}
        />

        <h3 className="mt-4 font-semibold text-slate-900">h. Catatan persetujuan dan audit</h3>
        <p className="mt-1">
          Kami mencatat kapan dan versi persetujuan apa yang Anda berikan, serta catatan audit atas tindakan penting di sistem (misalnya
          perubahan data, persetujuan perangkat, reset wajah, dan ekspor data) untuk akuntabilitas dan keamanan.
        </p>
      </>
    ),
  },
  {
    id: "tujuan",
    judul: "Tujuan Penggunaan Data",
    isi: (
      <>
        <Tabel
          kepala={["Tujuan", "Data yang dipakai", "Dasar pemrosesan"]}
          baris={[
            ["Mengelola akun dan masuk ke sistem", "Identitas, akun, sesi, perangkat", "Pelaksanaan hubungan kerja"],
            [
              "Mencatat presensi secara akurat dan mencegah titip absen",
              "Wajah, lokasi saat presensi, perangkat, presensi",
              "Persetujuan eksplisit (biometrik dan lokasi) serta pelaksanaan hubungan kerja",
            ],
            ["Memproses cuti, izin, dan persetujuan berjenjang", "Identitas, pengajuan, struktur organisasi", "Pelaksanaan hubungan kerja"],
            ["Menghitung dan menyusun rekap ubar", "Kepegawaian, ubar pokok, presensi", "Pelaksanaan hubungan kerja dan kewajiban hukum"],
            ["Administrasi data keluarga Pejuang", "Status pernikahan dan data anak", "Pelaksanaan hubungan kerja"],
            ["Menyampaikan pengumuman Lembaga", "Identitas, status baca", "Kepentingan sah Lembaga"],
            [
              "Menjaga keamanan sistem dan menyelidiki penyalahgunaan",
              "Sesi, IP, perangkat, hasil pemeriksaan integritas, audit",
              "Kepentingan sah Lembaga dan kewajiban hukum",
            ],
          ]}
        />
        <p className="mt-3">
          Kami <strong>tidak</strong> menjual data pribadi, tidak memakainya untuk iklan, dan tidak memakainya untuk tujuan di luar yang
          dijelaskan di sini tanpa memberi tahu Anda terlebih dahulu.
        </p>
      </>
    ),
  },
  {
    id: "wajah-lokasi",
    judul: "Cara Kerja Verifikasi Wajah dan Lokasi",
    isi: (
      <>
        <Daftar
          items={[
            "Sebelum mendaftarkan wajah, aplikasi menampilkan penjelasan dan meminta persetujuan Anda untuk pemrosesan data wajah dan lokasi.",
            "Pendeteksian posisi wajah di kamera berjalan langsung di HP Anda (Google ML Kit, di perangkat). Foto tidak dikirim ke Google.",
            "Foto selfie dikirim ke server Lembaga melalui koneksi terenkripsi. Server mencocokkan wajah dengan template milik akun Anda saja (pencocokan satu-ke-satu), bukan dengan seluruh Pejuang.",
            "Template wajah disimpan terenkripsi (AES-256-GCM) dan foto selfie disimpan terenkripsi di penyimpanan privat server.",
            "Presensi yang hasilnya meragukan tidak langsung ditolak secara otomatis, tetapi ditandai untuk ditinjau oleh staf SDM. Anda dapat meminta penjelasan atas hasil peninjauan.",
            "Lokasi dibandingkan dengan area kerja (geofence) yang ditetapkan SDM. Untuk Pejuang dengan pengaturan WFA atau hybrid, aturan lokasi menyesuaikan pengaturan kerjanya.",
          ]}
        />
      </>
    ),
  },
  {
    id: "berbagi",
    judul: "Pihak yang Dapat Mengakses Data",
    isi: (
      <>
        <p>Akses dibatasi sesuai peran (prinsip hak akses minimum):</p>
        <Daftar
          items={[
            <>
              <strong>Pejuang</strong> melihat data dirinya sendiri.
            </>,
            <>
              <strong>Atasan</strong> (kepala subdivisi dan kepala divisi) melihat presensi dan pengajuan Pejuang di unitnya, untuk keperluan
              persetujuan. Atasan tidak melihat data anak, data wajah, maupun ubar.
            </>,
            <>
              <strong>Staf SDM</strong> mengelola data kepegawaian, meninjau presensi berisiko (termasuk melihat foto selfie terkait, dan
              setiap akses tercatat), menyetujui perangkat, serta mengatur ubar.
            </>,
            <>
              <strong>Administrator sistem (PUTI)</strong> memelihara sistem dan hanya mengakses data bila diperlukan untuk dukungan teknis
              atau keamanan.
            </>,
          ]}
        />
        <p className="mt-3">Kami menggunakan layanan pihak ketiga berikut secara terbatas:</p>
        <Daftar
          items={[
            <>
              <strong>Google Play</strong> untuk distribusi aplikasi, dan <strong>Google Play Integrity API</strong> untuk memastikan
              aplikasi asli dan perangkat tidak dimodifikasi. Yang dikirim ke Google hanyalah token pemeriksaan, bukan foto, lokasi, atau
              data kepegawaian Anda.
            </>,
            <>
              <strong>OpenStreetMap</strong> untuk menampilkan peta dan mencari alamat saat SDM mengatur lokasi kantor di web admin. Layanan
              ini tidak menerima data presensi Pejuang.
            </>,
            <>
              <strong>Penyedia server (hosting)</strong> tempat sistem dijalankan, sebagai pemroses data yang wajib menjaga kerahasiaan
              data.
            </>,
          ]}
        />
        <p className="mt-3">
          Data juga dapat diungkapkan bila diwajibkan oleh peraturan perundang-undangan atau atas permintaan resmi aparat penegak hukum.
        </p>
      </>
    ),
  },
  {
    id: "penyimpanan",
    judul: "Penyimpanan dan Masa Simpan Data",
    isi: (
      <>
        <p>Data disimpan di server yang dikelola Lembaga. Kami menyimpan data hanya selama diperlukan:</p>
        <Tabel
          kepala={["Data", "Masa simpan"]}
          baris={[
            ["Foto selfie (pendaftaran dan presensi)", "Dihapus otomatis paling lama 90 hari setelah diambil"],
            ["Template wajah", "Selama Anda aktif sebagai Pejuang; dihapus paling lambat 30 hari setelah hubungan kerja berakhir"],
            ["Riwayat upaya presensi (termasuk koordinat lokasi)", "1 tahun"],
            ["Catatan audit", "2 tahun"],
            ["Data presensi, pengajuan, dan ubar", "Sesuai ketentuan Lembaga dan peraturan ketenagakerjaan serta perpajakan"],
            ["Data identitas, kepegawaian, dan keluarga", "Selama hubungan kerja, lalu diarsipkan sesuai ketentuan yang berlaku"],
            ["Cadangan (backup) basis data terenkripsi", "14 hari, lalu dihapus otomatis"],
          ]}
        />
      </>
    ),
  },
  {
    id: "keamanan",
    judul: "Keamanan Data",
    isi: (
      <Daftar
        items={[
          "Seluruh komunikasi antara aplikasi, web, dan server memakai HTTPS (TLS) terenkripsi. Aplikasi menolak koneksi tidak terenkripsi.",
          "Data wajah dan foto selfie dienkripsi di tingkat aplikasi; kata sandi disimpan sebagai hash argon2id.",
          "Token masuk di aplikasi disimpan di penyimpanan terenkripsi Android, dan web admin memakai cookie HttpOnly dengan perlindungan CSRF.",
          "Satu akun hanya terhubung ke satu perangkat yang disetujui. Perangkat baru memerlukan persetujuan SDM, dan perangkat yang hilang dapat dicabut beserta seluruh sesinya.",
          "Akun dikunci sementara setelah beberapa kali gagal masuk, dan web admin keluar otomatis setelah tidak aktif.",
          "Hak akses dibatasi per peran dan per unit kerja; tindakan penting tercatat di log audit yang tidak dapat diubah.",
          "Cadangan basis data dienkripsi dan diuji pemulihannya secara berkala.",
        ]}
      />
    ),
  },
  {
    id: "hak",
    judul: "Hak Anda sebagai Subjek Data",
    isi: (
      <>
        <p>Sesuai UU PDP, Anda berhak untuk:</p>
        <Daftar
          items={[
            "Mendapatkan informasi tentang tujuan dan dasar pemrosesan data pribadi Anda.",
            "Mengakses dan memperoleh salinan data pribadi Anda.",
            "Memperbaiki atau memperbarui data yang tidak akurat.",
            "Meminta penghapusan data, kecuali data yang wajib disimpan berdasarkan peraturan perundang-undangan.",
            "Menarik persetujuan pemrosesan data wajah dan lokasi. Setelah persetujuan ditarik, data wajah Anda dinonaktifkan dan presensi dilakukan melalui mekanisme lain yang ditetapkan SDM.",
            "Mengajukan keberatan atas keputusan yang semata-mata didasarkan pada pemrosesan otomatis, termasuk hasil verifikasi presensi.",
            "Meminta pembatasan pemrosesan dan menyampaikan keluhan atas pelanggaran pelindungan data pribadi.",
          ]}
        />
        <p className="mt-3">
          Permintaan disampaikan melalui saluran pada bagian Kontak. Kami akan memverifikasi identitas Anda lalu menindaklanjuti
          permintaan paling lambat <strong>3 x 24 jam</strong> sejak permintaan lengkap diterima, sesuai ketentuan UU PDP.
        </p>
      </>
    ),
  },
  {
    id: "insiden",
    judul: "Penanganan Kegagalan Pelindungan Data",
    isi: (
      <p>
        Bila terjadi kegagalan pelindungan data pribadi, kami akan segera membatasi dampaknya, menyelidiki penyebabnya, memulihkan sistem,
        dan memberitahukan Pejuang yang terdampak serta lembaga yang berwenang paling lambat <strong>3 x 24 jam</strong>, sesuai ketentuan
        UU PDP. Pemberitahuan memuat data yang terdampak, waktu dan cara kejadian, serta langkah penanganan yang dilakukan.
      </p>
    ),
  },
  {
    id: "rencana",
    judul: "Fitur yang Sedang Dikembangkan",
    isi: (
      <>
        <p>
          Sistem HRIS Al Bahjah terus dikembangkan. Fitur berikut direncanakan dan akan memproses data tambahan dengan prinsip yang sama
          seperti dijelaskan di kebijakan ini:
        </p>
        <Daftar
          items={[
            "Foto profil serta pembaruan nomor WhatsApp, email, dan data anak oleh Pejuang sendiri dari aplikasi.",
            "Lampiran bukti cuti dan izin (misalnya surat dokter) berupa JPEG, PNG, atau PDF, yang disimpan privat dan hanya dapat dilihat oleh pihak yang menyetujui serta SDM.",
            "Slip ubar dan proses penggajian bulanan, pengajuan kasbon, pengajuan mutasi, serta surat peringatan.",
            "Notifikasi ke HP melalui layanan notifikasi Google (Firebase Cloud Messaging), yang hanya berisi pemberitahuan singkat tanpa data sensitif.",
            "Pengiriman email untuk pemulihan akun dan pemberitahuan penting.",
          ]}
        />
        <p className="mt-3">
          Bila fitur tersebut mengubah jenis data atau tujuan pemrosesan secara berarti, kebijakan ini akan diperbarui dan, bila
          diperlukan, kami akan meminta persetujuan Anda kembali.
        </p>
      </>
    ),
  },
  {
    id: "anak",
    judul: "Pengguna di Bawah Umur",
    isi: (
      <p>
        Aplikasi ini ditujukan bagi Pejuang berusia 18 tahun ke atas dan tidak ditujukan untuk anak-anak. Data anak yang tercatat di sistem
        adalah data keluarga yang disampaikan oleh Pejuang sebagai orang tua atau wali, dan hanya dipakai untuk keperluan administrasi
        kepegawaian.
      </p>
    ),
  },
  {
    id: "perubahan",
    judul: "Perubahan Kebijakan",
    isi: (
      <p>
        Kebijakan ini dapat diperbarui sewaktu-waktu. Versi terbaru selalu tersedia di halaman ini dengan tanggal berlaku yang diperbarui.
        Perubahan penting akan kami umumkan melalui fitur pengumuman di aplikasi.
      </p>
    ),
  },
  {
    id: "kontak",
    judul: "Kontak",
    isi: (
      <>
        <p>Untuk pertanyaan, permintaan terkait hak Anda, atau laporan dugaan penyalahgunaan data, silakan menghubungi:</p>
        <Daftar
          items={[
            <>
              <strong>Bagian SDM</strong> Lembaga Pengembangan Dakwah Al-Bahjah, untuk data kepegawaian, presensi, dan pengajuan.
            </>,
            <>
              <strong>PUTI (Pusat Teknologi Informasi) Al Bahjah</strong>, untuk pertanyaan teknis dan keamanan sistem.
            </>,
            <>
              Situs resmi:{" "}
              <a href="https://albahjah.or.id" className="font-medium text-primary-700 underline" rel="noopener">
                albahjah.or.id
              </a>
            </>,
          ]}
        />
      </>
    ),
  },
];

export default function KebijakanPrivasiPage() {
  return (
    <main className="min-h-screen bg-slate-50 px-4 py-10">
      <article className="mx-auto max-w-3xl rounded-lg border border-slate-200 bg-white p-6 text-[15px] leading-relaxed text-slate-700 shadow-sm sm:p-10">
        <div className="mb-6">
          <Logo />
        </div>
        <h1 className="text-2xl font-bold text-slate-900">Kebijakan Privasi HRIS Al Bahjah</h1>
        <p className="mt-1 text-sm text-slate-500">Berlaku sejak {BERLAKU}</p>

        <nav aria-label="Daftar isi" className="mt-6 rounded-lg bg-slate-50 p-4 text-sm">
          <p className="font-semibold text-slate-900">Daftar isi</p>
          <ol className="mt-2 list-decimal space-y-1 pl-5">
            {bagian.map((b) => (
              <li key={b.id}>
                <a href={`#${b.id}`} className="text-primary-700 hover:underline">
                  {b.judul}
                </a>
              </li>
            ))}
          </ol>
        </nav>

        {bagian.map((b, i) => (
          <section key={b.id} id={b.id} className="mt-8 scroll-mt-6">
            <h2 className="text-lg font-semibold text-slate-900">
              {i + 1}. {b.judul}
            </h2>
            <div className="mt-2">{b.isi}</div>
          </section>
        ))}

        <p className="mt-10 border-t border-slate-200 pt-4 text-sm text-slate-500">
          © {new Date().getFullYear()} Lembaga Pengembangan Dakwah Al-Bahjah ·{" "}
          <Link href="/login/" className="text-primary-700 hover:underline">
            Masuk ke HRIS
          </Link>
        </p>
      </article>
    </main>
  );
}
