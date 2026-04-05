import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"

interface UserSummary {
  id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface SearchBarProps {
  value: string
  onChange: (q: string) => void
  results: UserSummary[]
  onAdd: (userId: string) => void
  onNavigate?: (username: string) => void
}

export default function SearchBar({ value, onChange, results, onAdd, onNavigate }: SearchBarProps) {
  return (
    <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
      <h2 className="mb-3 text-lg font-semibold text-white">Add Contact</h2>
      <Input
        label="Search contacts"
        labelHidden
        id="contact-search"
        type="text"
        placeholder="Search by name or email..."
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      {results.length > 0 && (
        <ul className="mt-3 divide-y divide-gray-700">
          {results.map((u) => (
            <li key={u.id} className="flex items-center justify-between py-2">
              <button
                type="button"
                onClick={() => onNavigate?.(u.username || u.id)}
                className="flex items-center gap-3 hover:opacity-80 disabled:pointer-events-none"
                disabled={!onNavigate}
                aria-label={`View profile of ${u.display_name || u.email}`}
              >
                <Avatar src={u.avatar_url} name={u.display_name || u.email} size="md" />
                <span className="text-sm text-gray-200">{u.display_name || u.email}</span>
              </button>
              <Button variant="primary" size="sm" onClick={() => onAdd(u.id)}>Add</Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
