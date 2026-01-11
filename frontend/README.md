# Frontend — Circl

Next.js 15+ con App Router, React, TypeScript y Tailwind.

## Getting Started

Instala dependencias y ejecuta el servidor de desarrollo:

```bash
npm install
npm run dev
```

Abre [http://localhost:3000](http://localhost:3000) en tu navegador.

Puedes editar `app/page.tsx` y la página se actualizará automáticamente.

## Scripts
- `npm run dev`: desarrollo
- `npm run build`: build para producción
- `npm run start`: inicia servidor prod
- `npm run lint`: ESLint
- `npm run format`: Prettier (agregar después)
- `npm test`: Jest/Vitest (agregar después)

## Estructura
```
/app              # Next.js App Router (rutas y páginas)
/components       # Componentes React reutilizables
/hooks            # Custom hooks
/lib              # Utilidades, API client, auth helpers
/public           # Assets estáticos
```

## Variables de entorno
Ver `.env.example` y copia a `.env.local`:
```bash
cp .env.example .env.local
```

## Más información
- [Next.js Documentation](https://nextjs.org/docs)
- [Learn Next.js](https://nextjs.org/learn)
- [Tailwind CSS](https://tailwindcss.com/docs)