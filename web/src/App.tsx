import {useState} from 'react'

type Role = 'rider' | 'driver'
type Status = 'idle'| 'open' | 'error' 


const API = 'http://127.0.0.1:8080'

export default function App() {

const [role, setRole] = useState<Role>('rider')
const [bookingId, setBookingId] = useState<string>("")
const [status, setStatus] = useState<Status>('idle') // for safety reasons it needs to be off by default
const [errMsg, setErrMsg] = useState<string>("")

  async function openSession() {
    try {
      const res = await fetch(`{API}/sessions`, {
        method: 'POST',
        headers: {
         'Content-Type': 'application/json',
         'X-User-ID': role === 'rider' ? 'rider' : 'driver',
        },
        body: JSON.stringify({ bookingId, riderId: 'r1', driverId: 'd1', ttlSeconds: 60})
        })
        if (res.status === 201 ) {
          setStatus('open')
        } else {
          setStatus('error')
          setErrMsg(`HTTP ${"res.status"}`)
        }
      }
      
      catch {
        setStatus('error')
        setErrMsg('network / CORS')
      }

  }

  const pill = (r:Role) =>
    `rounded-xl py-3 font-medium transition ${role === r ? 'bg-emerald-500 text-neutral-950' : 'bg-neutral-800 text-neutral-300'}`

  return (
    <main className="min-h-dvh bg-neutral-950 text-neutral-100 flex items-center justify-center p-4">
      <div className="w-full max-w-sm space-y-6">
        <header>
          <p className="text-xs uppercase tracking-widest text-emerald-400">Pickup window</p>
          <h1 className="text-3xl font-semibold">Find each other</h1>
        </header>

        <div className="grid grid-cols-2 gap-2">
          <button className={pill('rider')} onClick={() => setRole('rider')}>Rider</button>
          <button className={pill('driver')} onClick={() => setRole('driver')}>Driver</button>
        </div>

        <input
          className="w-full rounded-xl bg-neutral-900 border border-neutral-800 px-4 py-3 outline-none focus:border-emerald-500"
          placeholder="Booking ID (e.g. b1)"
          value={bookingId}
          onChange={(e) => setBookingId(e.target.value)}
        />

        <button
          className="w-full rounded-xl bg-emerald-500 py-3 font-semibold text-neutral-950 disabled:opacity-40"
          disabled={!bookingId}
          onClick={openSession}
        >
          Start pickup session
        </button>

        {status === 'open' && <p className="text-emerald-400">Session open · {role} · {bookingId}</p>}
        {status === 'error' && <p className="text-red-400">{errMsg}</p>}
      </div>
    </main>
  )
}









