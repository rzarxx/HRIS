// Data dummy realistis untuk pengembangan & demo (CHECKLIST Fase 0).
// Deterministik: seed yang sama menghasilkan data yang sama di setiap reset.
import type { Announcement, Division, Employee, Subdivision } from "@/lib/types";
import { hashPassword } from "./password";
import type { DB, ShiftRow, UserRow } from "./store";

export const DEMO_SDM_EMAIL = "sdm@pejuang.id";
export const DEMO_SDM_PASSWORD = process.env.SEED_SDM_PASSWORD ?? "pejuang123";

/** PRNG kecil (mulberry32) agar data stabil antar-reset. */
export function rng(seed: number) {
  return () => {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const ORG: { code: string; name: string; subs: { code: string; name: string; size: number; positions: string[]; salary: [number, number] }[] }[] = [
  {
    code: "OPS",
    name: "Operasional",
    subs: [
      { code: "OPS-LAP", name: "Lapangan", size: 12, positions: ["Staf Lapangan", "Koordinator Lapangan"], salary: [3_800_000, 5_200_000] },
      { code: "OPS-GDG", name: "Gudang", size: 8, positions: ["Staf Gudang", "Admin Gudang"], salary: [3_600_000, 4_800_000] },
      { code: "OPS-DST", name: "Distribusi", size: 6, positions: ["Kurir", "Pengemudi", "Admin Distribusi"], salary: [3_700_000, 5_000_000] },
      { code: "OPS-KAM", name: "Keamanan", size: 5, positions: ["Petugas Keamanan"], salary: [3_500_000, 4_200_000] },
    ],
  },
  {
    code: "KEU",
    name: "Keuangan",
    subs: [
      { code: "KEU-AKT", name: "Akuntansi", size: 5, positions: ["Staf Akuntansi", "Akuntan"], salary: [5_000_000, 7_500_000] },
      { code: "KEU-PJK", name: "Pajak", size: 3, positions: ["Staf Pajak"], salary: [5_200_000, 7_000_000] },
      { code: "KEU-TGH", name: "Penagihan", size: 4, positions: ["Staf Penagihan", "Kolektor"], salary: [4_200_000, 6_000_000] },
    ],
  },
  {
    code: "IT",
    name: "Teknologi Informasi",
    subs: [
      { code: "IT-DEV", name: "Pengembangan", size: 6, positions: ["Programmer", "QA Engineer", "UI/UX Designer"], salary: [6_500_000, 9_500_000] },
      { code: "IT-INF", name: "Infrastruktur", size: 3, positions: ["Network Engineer", "IT Support"], salary: [5_500_000, 8_000_000] },
    ],
  },
  {
    code: "SDM",
    name: "Sumber Daya Manusia",
    subs: [
      { code: "SDM-PRS", name: "Personalia", size: 3, positions: ["Staf Personalia"], salary: [5_000_000, 7_000_000] },
      { code: "SDM-UMM", name: "Umum", size: 3, positions: ["Staf Umum", "Resepsionis"], salary: [3_800_000, 5_000_000] },
    ],
  },
];

const FIRST = [
  "Agus", "Budi", "Citra", "Dewi", "Eko", "Fajar", "Gita", "Hendra", "Indah", "Joko", "Kartika", "Lukman",
  "Maya", "Nanda", "Oki", "Putri", "Rizky", "Sari", "Taufik", "Utami", "Wahyu", "Yuni", "Zainal", "Ayu",
  "Bayu", "Dimas", "Endah", "Fitri", "Galih", "Hana", "Irfan", "Jihan", "Kurnia", "Lestari", "Rahmat", "Nur",
  "Andi", "Rina", "Teguh", "Wulan", "Arif", "Dian", "Siti", "Yoga", "Ratna", "Bagus", "Novi", "Ilham",
];
const LAST = [
  "Santoso", "Wijaya", "Pratama", "Saputra", "Lestari", "Hidayat", "Kusuma", "Nugroho", "Rahayu", "Setiawan",
  "Siregar", "Permana", "Ramadhan", "Hakim", "Purnomo", "Maharani", "Syahputra", "Wibowo", "Anggraini", "Firmansyah",
];

/** UUID v4 dari PRNG, agar ID tetap sama setelah reset (sesi login tetap berlaku). */
function seededUuid(rand: () => number) {
  const hex = Array.from({ length: 32 }, () => Math.floor(rand() * 16).toString(16));
  hex[12] = "4";
  hex[16] = ((parseInt(hex[16], 16) & 0x3) | 0x8).toString(16);
  const h = hex.join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export function buildSeed(): Omit<DB, "version"> {
  const rand = rng(20261102);
  const idRand = rng(7);
  const newId = () => seededUuid(idRand);
  const pick = <T,>(arr: T[]) => arr[Math.floor(rand() * arr.length)];
  const created = "2026-10-01T01:00:00.000Z";

  const divisions: Division[] = [];
  const subdivisions: Subdivision[] = [];
  const employees: Employee[] = [];
  const usedNames = new Set<string>();
  let seq = 1;

  for (const div of ORG) {
    const division: Division = { id: newId(), code: div.code, name: div.name, head_employee_id: null, created_at: created };
    divisions.push(division);

    for (const sub of div.subs) {
      const subdivision: Subdivision = {
        id: newId(),
        division_id: division.id,
        code: sub.code,
        name: sub.name,
        head_employee_id: null,
        created_at: created,
      };
      subdivisions.push(subdivision);

      for (let i = 0; i < sub.size; i++) {
        let name = "";
        do name = `${pick(FIRST)} ${pick(LAST)}`;
        while (usedNames.has(name));
        usedNames.add(name);

        const year = 2017 + Math.floor(rand() * 9);
        const month = 1 + Math.floor(rand() * 12);
        const salary = Math.round((sub.salary[0] + rand() * (sub.salary[1] - sub.salary[0])) / 50_000) * 50_000;
        const isHead = i === 0;
        const emp: Employee = {
          id: newId(),
          nip: `PJ${year}${String(seq++).padStart(4, "0")}`,
          full_name: name,
          email: `${name.toLowerCase().replace(/\s+/g, ".")}@pejuang.id`,
          phone: `08${Math.floor(1_000_000_000 + rand() * 8_999_999_999)}`,
          subdivision_id: subdivision.id,
          position: isHead ? `Kepala Subdiv ${sub.name}` : pick(sub.positions),
          join_date: `${year}-${String(month).padStart(2, "0")}-${String(1 + Math.floor(rand() * 27)).padStart(2, "0")}`,
          base_salary: isHead ? salary + 1_500_000 : salary,
          photo_url: null,
          status: "active",
          created_at: created,
          updated_at: created,
          deleted_at: null,
        };
        employees.push(emp);
        if (isHead) subdivision.head_employee_id = emp.id;
      }
    }
    // Kepala divisi = kepala subdiv pertama di divisinya
    const firstSub = subdivisions.find((s) => s.division_id === division.id);
    division.head_employee_id = firstSub?.head_employee_id ?? null;
    const head = employees.find((e) => e.id === division.head_employee_id);
    if (head) head.position = `Kepala Divisi ${div.name}`;
  }

  // Satu Pejuang nonaktif sebagai contoh status
  employees[employees.length - 4].status = "inactive";

  // Akun SDM menempel ke salah satu staf Personalia (users.employee_id wajib terisi)
  const personalia = subdivisions.find((s) => s.code === "SDM-PRS")!;
  const sdmEmployee = employees.find((e) => e.subdivision_id === personalia.id && e.id !== personalia.head_employee_id)!;
  sdmEmployee.position = "Admin SDM";
  const users: UserRow[] = [
    {
      id: newId(),
      employee_id: sdmEmployee.id,
      email: DEMO_SDM_EMAIL,
      password_hash: hashPassword(DEMO_SDM_PASSWORD),
      role: "sdm",
      is_active: true,
      last_login_at: null,
      created_at: created,
    },
  ];

  const shifts: ShiftRow[] = [
    { id: newId(), name: "Reguler", start_time: "08:00", end_time: "17:00", late_tolerance_min: 10 },
  ];

  const opsId = divisions.find((d) => d.code === "OPS")!.id;
  const announcements: Announcement[] = [
    {
      id: newId(),
      title: "Selamat datang di HRIS Pejuang",
      body: "Mulai bulan ini presensi, pengajuan cuti, dan slip upah dapat diakses lewat aplikasi HRIS Pejuang.",
      type: "popup",
      image_url: null,
      division_id: null,
      starts_at: "2026-10-28T01:00:00.000Z",
      ends_at: "2026-11-30T16:59:00.000Z",
      is_active: true,
      created_by: users[0].id,
      created_at: "2026-10-27T03:00:00.000Z",
    },
    {
      id: newId(),
      title: "Briefing keselamatan kerja",
      body: "Seluruh Pejuang divisi Operasional wajib mengikuti briefing K3 setiap Senin pukul 07.45 WIB di halaman gudang.",
      type: "banner",
      image_url: null,
      division_id: opsId,
      starts_at: "2026-10-20T00:00:00.000Z",
      ends_at: null,
      is_active: true,
      created_by: users[0].id,
      created_at: "2026-10-19T08:00:00.000Z",
    },
    {
      id: newId(),
      title: "Upacara Hari Pahlawan",
      body: "Tanggal 10 November tetap masuk kerja seperti biasa. Upacara singkat pukul 08.00 WIB.",
      type: "banner",
      image_url: null,
      division_id: null,
      starts_at: "2026-11-05T00:00:00.000Z",
      ends_at: "2026-11-10T16:59:00.000Z",
      is_active: true,
      created_by: users[0].id,
      created_at: "2026-10-30T02:00:00.000Z",
    },
  ];

  return { divisions, subdivisions, employees, users, shifts, attendances: [], announcements };
}
