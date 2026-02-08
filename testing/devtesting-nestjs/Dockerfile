FROM node:20-alpine AS builder

WORKDIR /app

COPY tracewayapp-core.tgz tracewayapp-backend.tgz tracewayapp-nestjs.tgz ./

COPY package.json ./
RUN node -e " \
  const pkg = JSON.parse(require('fs').readFileSync('package.json', 'utf8')); \
  pkg.dependencies['@tracewayapp/core'] = 'file:./tracewayapp-core.tgz'; \
  pkg.dependencies['@tracewayapp/backend'] = 'file:./tracewayapp-backend.tgz'; \
  pkg.dependencies['@tracewayapp/nestjs'] = 'file:./tracewayapp-nestjs.tgz'; \
  require('fs').writeFileSync('package.json', JSON.stringify(pkg, null, 2)); \
"

RUN npm install

COPY tsconfig.json nest-cli.json ./
COPY src/ ./src/

RUN npm run build

# ---

FROM node:20-alpine

RUN npm install -g pm2

WORKDIR /app

COPY tracewayapp-core.tgz tracewayapp-backend.tgz tracewayapp-nestjs.tgz ./

COPY package.json ./
RUN node -e " \
  const pkg = JSON.parse(require('fs').readFileSync('package.json', 'utf8')); \
  pkg.dependencies['@tracewayapp/core'] = 'file:./tracewayapp-core.tgz'; \
  pkg.dependencies['@tracewayapp/backend'] = 'file:./tracewayapp-backend.tgz'; \
  pkg.dependencies['@tracewayapp/nestjs'] = 'file:./tracewayapp-nestjs.tgz'; \
  require('fs').writeFileSync('package.json', JSON.stringify(pkg, null, 2)); \
"

RUN npm install --omit=dev

COPY --from=builder /app/dist ./dist
COPY ecosystem.config.js ./

EXPOSE 3001

CMD ["pm2-runtime", "ecosystem.config.js"]
