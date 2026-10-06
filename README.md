# Pixeluxe

Pixeluxe est un éditeur de pixel art en Go inspiré de **Deluxe Paint II sur Amiga**. Il reprend la barre Picture / Brush / Mode / Effects / Font / Prefs, la boîte à outils à droite et les images et fontes bitmap du disque fourni. Les opérations travaillent sur des pixels indexés et leur palette ; le programme Amiga n'est pas exécuté ni émulé.

![Pixeluxe avec l'image Seascape originale](docs/pixeluxe.png)

## Démarrer

Prérequis : **Go 1.25 ou ultérieur**. La fenêtre utilise **Ebitengine 2.10.2** ; la compilation de l'application de bureau s'effectue avec `CGO_ENABLED=0`, sans compilateur C. Le premier lancement télécharge les dépendances Go. Les exemples et les fontes sont intégrés au binaire.

```sh
CGO_ENABLED=0 go run .
```

Pour construire l'exécutable puis ouvrir l'exemple original :

```sh
make build
./bin/pixeluxe -demo
```

Sur macOS, créer et ouvrir le paquet d'application :

```sh
make app
open bin/Pixeluxe.app
```

Au démarrage normal, la page mesure 320 × 256 pixels avec 32 couleurs. L'interface a une résolution logique de 640 × 512 ; la fenêtre est redimensionnable. `-scale 1`, `2` ou `3` règle sa taille initiale, et F11 active le plein écran.

## Ouvrir et enregistrer

Utiliser **Picture > Load**, Ctrl/Cmd+O, déposer un fichier sur la fenêtre, ou donner un chemin au lancement :

```sh
./bin/pixeluxe -open dessin.iff
./bin/pixeluxe dessin.png
```

Lecture : IFF ILBM/PBM, PNG, GIF et JPEG. Les images indexées conservent leurs indices et leur palette ; les images en couleurs directes sont converties en une palette d'au plus 32 couleurs. Pour un GIF animé, seule la première image est importée. Les ILBM EHB sont pris en charge ; le mode HAM est refusé explicitement.

**Picture > Save / Save As** ou Ctrl/Cmd+S enregistre en IFF ILBM, PNG ou GIF selon l'extension. Sans extension, le dialogue ajoute `.iff`. Ctrl/Cmd+Shift+S ouvre « Save As ». Les dialogues affichent le répertoire, permettent de remonter avec **Up** et de saisir **Drawer** et **File**. « Save As » confirme le remplacement d'un fichier existant ; l'abandon d'une image modifiée demande aussi confirmation. **Picture > Examples** ouvre Seascape, StencilSet ou Reference Palette ; **Print to PDF** exporte l'image sur une page A4 dans `pixeluxe-print.pdf`. La commande `./bin/pixeluxe -demo -pdf dessin.pdf` exporte aussi un PDF sans ouvrir de fenêtre.

**Page Size** et **Screen Format** changent les dimensions et le nombre de couleurs en conservant les pixels depuis le coin supérieur gauche ; les parties dépassant une page réduite sont coupées. Les couleurs supprimées sont remappées vers la palette retenue. L'opération est annulable. Les fichiers IFF conservent leurs plages de cyclage CRNG, leur point de prise GRAB et leur rapport de pixel lors d'un enregistrement IFF.

## Peindre

Dans la palette, clic gauche choisit la couleur de premier plan et clic droit la couleur de fond. Sur la page, ces boutons peignent respectivement avec ces couleurs. La pipette est accessible par `,` ou Alt+clic. Les menus acceptent le bouton droit maintenu avec sélection au relâchement, ainsi que le clic gauche.

Les outils comprennent dessin libre continu ou pointillé, droite, courbe, aérographe, remplissage et formes en contour ou pleines. Le clic droit sur une icône de forme choisit sa version pleine. Pour une courbe, tracer d'abord le segment, puis cliquer pour placer sa courbure. Pour un polygone, poser les sommets, puis terminer avec Entrée, clic droit ou clic sur le premier sommet. Shift contraint les lignes et les proportions des formes.

`b` puis un glissement rectangulaire capture une brosse : la couleur de fond devient transparente. **Brush > Load / Save** échange des brosses IFF ou PNG. **Original brushes** donne accès aux brosses Dolphin, Building et Pattern1 du disque, intégrées au binaire. Le panneau propose les dix brosses prédéfinies : quatre rondes, quatre carrées et deux pointillées. Le menu propose redimensionnement au plus proche voisin, retournements, rotation à 90° ou libre, cisaillement, courbure et projection en perspective. Les paramètres des transformations sont saisis dans des dialogues. Les remplissages, y compris les formes pleines, proposent couleur unie, motif de brosse et dégradés de plage avec ou sans tramage. Les contours utilisent la brosse, le mode et la symétrie sélectionnés.

Les huit modes Matte, Color, Replc, Smear, Shade, Blend, Cycle et Smooth sont présents. La grille peut attirer le curseur ; la symétrie propose miroirs horizontal/vertical et copies radiales. La page de réserve permet de copier, échanger et fusionner des images. **Effects > Stencil** crée un masque depuis les couleurs sélectionnées, le refait, l'inverse ou le désactive. Il charge et enregistre aussi le masque en IFF/PNG à deux couleurs. Les menus Picture, Brush et Stencil proposent Delete, avec confirmation avant suppression du fichier. **Background > Fix** mémorise l'image courante : le bouton droit et CLR restaurent ensuite ce fond sous les ajouts. **Lock FG** protège les zones peintes depuis sa fixation ; **Background > Off** rétablit l'effacement normal. L'historique des images et palettes offre jusqu'à 64 annulations, avec rétablissement.

Tab anime la palette sans changer les indices des pixels. Les plages CRNG d'un IFF sont lues avec leur vitesse et leur sens ; **Prefs > MultiCycle** permet de les animer simultanément. **Color Ranges** règle la plage active utilisée pour les dégradés et le mode Cycle.

**Prefs > Fast FB** affiche les contours provisoires avec une brosse d'un pixel puis applique la brosse choisie au tracé final. **ExclBrush**, avec attraction à la grille activée, exclut la dernière colonne et la dernière ligne lors d'une capture pour éviter de doubler les bordures des motifs répétés.

Pour le texte, sélectionner `t`, cliquer puis saisir ; Entrée pose le texte et Échap annule. **Font** donne accès aux 14 fontes originales extraites : ruby, opal, sapphire, diamond, garnet, emerald et topaz, dans leurs tailles disque. Gras, italique, soulignement et agrandissement entier sont disponibles.

## Raccourcis

Les lettres majuscules correspondent à Shift. Cette table décrit le comportement actuel de Pixeluxe ; les différences avec les commandes Amiga sont précisées ensuite.

| Touche | Action |
|---|---|
| `s`, `d`, `D` | Dessin pointillé, continu, continu avec brosse d'un pixel |
| `v`, `q`, `f`, `F` | Droite, courbe, remplissage, réglages de remplissage |
| `r` / `R`, `c` / `C`, `e` / `E` | Rectangle, cercle, ellipse : contour / plein |
| `b`, `B`, `t` | Capturer une brosse, reprendre la dernière brosse, texte |
| `u`, `U`, `K` | Annuler, rétablir, effacer avec la couleur de fond |
| `p`, `j`, Tab | Palette, échanger la page de réserve, activer/désactiver le cyclage |
| `x`, `y`, `z`, `Z`, `h`, `H` | Retourner X/Y, rotation 90°, taille de brosse, moitié, double |
| F1…F8 | Matte, Color, Replc, Smear, Shade, Blend, Cycle, Smooth |
| `g`, `G`, `/` | Attraction à la grille, réglages de grille, miroir horizontal |
| `m`, `<`, `>` | Outil loupe, réduire/agrandir le zoom |
| `,`, `.`, `-`, `=` | Pipette, brosse d'un pixel, réduire/agrandir une brosse prédéfinie |
| `[`, `]` | Couleur précédente/suivante dans la palette entière |
| `n`, flèches | Centrer la page, déplacer la vue |
| `S`, F9, F10, F11 | Vue sans outils avec zoom remis à 1, masquer la barre, masquer barre et outils, plein écran |
| `a` | Répéter la dernière commande de menu |
| Échap, Espace | Annuler le geste en cours ; Espace+glissement déplace la vue |
| Entrée | Terminer le polygone ou poser le texte |

Extensions de bureau : Ctrl/Cmd+N/O/S pour nouveau/ouvrir/enregistrer, Ctrl/Cmd+Q pour quitter, Ctrl/Cmd+Z pour annuler, Ctrl/Cmd+Shift+Z ou Ctrl/Cmd+Y pour rétablir. La molette zoome ; le bouton central ou Espace+glissement déplace la vue. L'aide est aussi dans **Prefs > Keyboard Help**.

## Fidélité et limites

Pixeluxe est une réimplémentation fonctionnelle, avec des adaptations de l'interface et des algorithmes ; la parité complète avec Deluxe Paint II 2.0P n'est pas revendiquée.

- Les dialogues de fichiers, palette et paramètres reprennent le style Amiga, avec des interactions et une disposition propres à Pixeluxe. Les titres des menus restent visibles au repos. `G` ouvre les réglages au lieu d'aligner la grille sur le pinceau, `n` centre la page et `S` ne reproduit pas l'aperçu réduit de l'original.
- L'éditeur de plages règle une plage active ; il ne reprend pas le panneau original C1–C4 avec tous ses réglages. MultiCycle anime plusieurs plages à l'affichage, sans garantir le comportement original du mode Cycle sur les brosses multicolores. Les métadonnées IFF de perspective et l'ensemble des chunks propriétaires ne sont pas conservés. Les exports PNG/GIF ne transportent pas les métadonnées Amiga.
- Be Square et l'intégration Workbench restent absents ; l'impression passe par un PDF et ne reprend pas les pilotes et réglages d'imprimante Amiga.
- La perspective transforme directement une brosse avec des angles X/Y/Z. Elle ne reprend pas la grille interactive au pavé numérique, le centre de perspective, FillScreen ou les options d'anticrénelage de Deluxe Paint. Smear, Shade, Blend et Smooth utilisent des algorithmes adaptés ; leur résultat n'est pas garanti identique au logiciel Amiga.
- La fonte système Topaz 8 de la ROM n'est pas présente sur le disque. L'interface utilise une fonte compacte de remplacement ; les 14 fontes de texte proviennent réellement de l'ADF. La palette initiale de Pixeluxe suit la grille RGB 12 bits Amiga, mais n'a pas été confirmée comme la palette exacte au démarrage de cette version.

Le disque d'origine reste dans [`previous/`](previous/). L'extraction, les faits confirmés, les palettes, les planches de fontes et les limites de la recherche sont documentés dans [`reference/DELUXE_PAINT_II.md`](reference/DELUXE_PAINT_II.md), avec le [manuel primaire Electronic Arts](https://d1yx3ys82bpsa0.cloudfront.net/atchm/documents/DeluxePaint_II_manual.pdf).

## Vérifier et produire une capture

```sh
make test
make check
make preview
```

`make test` exécute les tests Go avec CGO désactivé ; `make check` ajoute `go vet`. Les vérifications couvrent notamment formats indexés, transparence, données malformées, transformations, historique, fontes et dialogues. `make preview` écrit le rendu de démonstration dans [`docs/pixeluxe.png`](docs/pixeluxe.png), sans ouvrir de fenêtre.

Pour vérifier aussi l'ouverture de la fenêtre et la boucle Ebitengine pendant 120 images :

```sh
./bin/pixeluxe -demo -smoke 120 -capture /tmp/pixeluxe-smoke.png
```

Le programme quitte automatiquement après la capture. Les tests du moteur sont distincts de cette vérification de bureau. Un lancement natif de 120 images a également produit [`docs/desktop-smoke.png`](docs/desktop-smoke.png).
